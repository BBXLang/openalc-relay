package org.openalc.relay.poc

import java.io.File
import java.util.UUID
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking

// Repeatable multi-process verification without an Android SDK. This does not simulate OS delivery.
fun main(args: Array<String>) = runBlocking {
    require(args.size == 4) { "Expected mode, database path, loopback endpoint, and fixture count" }
    val mode = args[0]
    require(mode == "seed" || mode == "sync")
    val file = File(args[1]).absoluteFile
    file.parentFile.mkdirs()
    val database = openJvmDatabase(file.path)
    try {
        val outbox = database.outbox()
        outbox.initialize(UUID.randomUUID().toString())
        if (mode == "seed") {
            val count = args[3].toInt()
            require(count in 1..1000)
            repeat(count) { outbox.capture(UUID.randomUUID().toString(), "injected", System.currentTimeMillis(), null) }
        } else {
            SyncEngine(outbox, LoopbackTransport(args[2]), System::currentTimeMillis).synchronize()
        }
        val counts = outbox.observeCounts().first()
        val fixtureIds = if (mode == "seed") outbox.due(Long.MAX_VALUE, 1000).joinToString(",") { "\"${it.fixtureId}\"" } else ""
        println("{\"pending\":${counts.pending},\"synced\":${counts.synced},\"rejected\":${counts.rejected},\"installation_id\":\"${outbox.installationId()}\",\"fixture_ids\":[$fixtureIds]}")
    } finally {
        database.close()
    }
}
