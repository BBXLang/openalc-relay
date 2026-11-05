package org.openalc.relay.poc

import java.io.File
import java.nio.file.Files
import java.util.UUID
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class OutboxTest {
    private fun id() = UUID.randomUUID().toString()
    private val timestamp = 1793577600000L

    private fun withDatabase(block: suspend (PocDatabase, File) -> Unit) = runBlocking {
        val directory = Files.createTempDirectory("relay-synthetic-test").toFile()
        val file = File(directory, "outbox.db")
        val database = openJvmDatabase(file.absolutePath)
        try {
            database.outbox().initialize(id())
            block(database, file)
        } finally {
            database.close()
            directory.deleteRecursively()
        }
    }

    @Test
    fun strictFixtureAllowlistDiscardsOrdinarySms() {
        val fixture = id()
        assertEquals(fixture, syntheticFixtureId(listOf(SMS_PREFIX, fixture)))
        for (parts in listOf(emptyList(), listOf(null), listOf("synthetic-unrelated-text"), listOf(SMS_PREFIX + fixture + "\n"), listOf(SMS_PREFIX + "invalid"))) {
            assertNull(syntheticFixtureId(parts))
        }
        assertNull(syntheticFixtureId(listOf("x".repeat(200_000))))
    }

    @Test
    fun capturePreservesIdentityAndFirstPayloadAcrossReopen() = withDatabase { database, file ->
        val installation = database.outbox().installationId()
        val event = id()
        assertTrue(database.outbox().capture(event, "injected", timestamp, null))
        assertFalse(database.outbox().capture(event, "sms_broadcast", timestamp + 1, 5))
        database.close()
        val reopened = openJvmDatabase(file.absolutePath)
        try {
            assertEquals(installation, reopened.outbox().initialize(id()))
            val stored = assertNotNull(reopened.outbox().find(event))
            assertEquals(PENDING, stored.state)
            assertEquals(timestamp, stored.receivedAtMs)
            assertNull(stored.subscriptionId)
        } finally {
            reopened.close()
        }
    }

    @Test
    fun offlineThenSuccessfulSyncRetainsStableIdsForOneHundredFixtures() = withDatabase { database, _ ->
        val outbox = database.outbox()
        val ids = List(100) { id() }
        ids.forEach { outbox.capture(it, "injected", timestamp, null) }
        val offline = SyncEngine(outbox, CollectorTransport { UploadResult.Retry("network_unavailable") }) { timestamp }
        assertEquals(SyncOutcome.RETRY, offline.synchronize())
        assertEquals(100, outbox.pendingCount())
        val received = mutableSetOf<String>()
        val recovered = SyncEngine(outbox, CollectorTransport { batch ->
            UploadResult.Accepted(Receipt(1, batch.installationId, batch.events.map {
                Acknowledgement(it.eventId, if (received.add(it.eventId)) "persisted" else "duplicate")
            }))
        }) { timestamp }
        assertEquals(SyncOutcome.COMPLETE, recovered.synchronize())
        assertEquals(ids.toSet(), received)
        assertEquals(QueueCounts(0, 100, 0), outbox.observeCounts().first())
    }

    @Test
    fun lostAckAndDatabaseReopenReplayTheSameEvent() = withDatabase { database, file ->
        val event = id()
        database.outbox().capture(event, "injected", timestamp, 7)
        var committed: WireEvent? = null
        val first = SyncEngine(database.outbox(), CollectorTransport { batch ->
            committed = batch.events.single()
            UploadResult.Retry("network_unavailable")
        }) { timestamp }
        assertEquals(SyncOutcome.RETRY, first.synchronize())
        assertEquals(PENDING, database.outbox().find(event)?.state)
        database.close()
        val reopened = openJvmDatabase(file.absolutePath)
        try {
            val second = SyncEngine(reopened.outbox(), CollectorTransport { batch ->
                assertEquals(committed, batch.events.single())
                UploadResult.Accepted(Receipt(1, batch.installationId, listOf(Acknowledgement(event, "duplicate"))))
            }) { timestamp }
            assertEquals(SyncOutcome.COMPLETE, second.synchronize())
            assertEquals(SYNCED, reopened.outbox().find(event)?.state)
            assertEquals(2, reopened.outbox().find(event)?.attempts)
        } finally {
            reopened.close()
        }
    }

    @Test
    fun missingAckLeavesUnconfirmedRecordsPending() = withDatabase { database, _ ->
        val ids = List(2) { id() }
        ids.forEach { database.outbox().capture(it, "injected", timestamp, null) }
        val engine = SyncEngine(database.outbox(), CollectorTransport { batch ->
            UploadResult.Accepted(Receipt(1, batch.installationId, listOf(Acknowledgement(batch.events.first().eventId, "persisted"))))
        }) { timestamp }
        assertEquals(SyncOutcome.RETRY, engine.synchronize())
        assertEquals(QueueCounts(1, 1, 0), database.outbox().observeCounts().first())
    }

    @Test
    fun invalidAckCannotMarkAnyRecordSynced() = withDatabase { database, _ ->
        val event = id()
        val outbox = database.outbox()
        outbox.capture(event, "injected", timestamp, null)
        val invalidReceipts: List<(Batch) -> Receipt> = listOf(
            { Receipt(1, id(), listOf(Acknowledgement(event, "persisted"))) },
            { Receipt(1, it.installationId, listOf(Acknowledgement(id(), "persisted"))) },
            { Receipt(1, it.installationId, List(2) { Acknowledgement(event, "persisted") }) },
            { Receipt(1, it.installationId, listOf(Acknowledgement(event, "sent"))) },
            { Receipt(1, it.installationId, listOf(Acknowledgement(event, "rejected", "unexpected"))) },
            { Receipt(1, it.installationId, emptyList()) },
        )
        for (receipt in invalidReceipts) {
            val engine = SyncEngine(outbox, CollectorTransport { UploadResult.Accepted(receipt(it)) }) { timestamp }
            assertEquals(SyncOutcome.RETRY, engine.synchronize())
            assertEquals(PENDING, outbox.find(event)?.state)
            assertEquals("invalid_ack", outbox.find(event)?.lastError)
        }
    }

    @Test
    fun rejectionIsRetainedAndNotAutomaticallyRetried() = withDatabase { database, _ ->
        val event = id()
        database.outbox().capture(event, "injected", timestamp, null)
        var calls = 0
        val engine = SyncEngine(database.outbox(), CollectorTransport { batch ->
            calls++
            UploadResult.Accepted(Receipt(1, batch.installationId, listOf(Acknowledgement(event, "rejected", "event_id_conflict"))))
        }) { timestamp }
        repeat(2) { assertEquals(SyncOutcome.COMPLETE, engine.synchronize()) }
        assertEquals(1, calls)
        assertEquals(REJECTED, database.outbox().find(event)?.state)
        assertEquals("event_id_conflict", database.outbox().find(event)?.lastError)
    }

    @Test
    fun retryAfterSurvivesDatabaseReopenAndAvoidsEarlyNetworkRequests() = withDatabase { database, file ->
        val event = id()
        database.outbox().capture(event, "injected", timestamp, null)
        val delayed = SyncEngine(database.outbox(), CollectorTransport { UploadResult.Retry("http_retryable", timestamp + 60_000) }) { timestamp }
        assertEquals(SyncOutcome.RETRY, delayed.synchronize())
        database.close()
        val reopened = openJvmDatabase(file.absolutePath)
        try {
            val engine = SyncEngine(reopened.outbox(), CollectorTransport { error("network request before Retry-After") }) { timestamp + 59_000 }
            assertEquals(SyncOutcome.RETRY, engine.synchronize())
            assertEquals(timestamp + 60_000, reopened.outbox().find(event)?.nextAttemptAtMs)
        } finally {
            reopened.close()
        }
    }

    @Test
    fun sharedWireFixtureDecodesWithoutSensitiveFields() {
        val batch = protocolJson.decodeFromString<Batch>(File("../../../packages/protocol/fixtures/synthetic-batch.json").readText())
        assertEquals(FIXTURE, batch.fixture)
        assertEquals(batch.events.single().fixtureId, batch.events.single().eventId)
    }
}
