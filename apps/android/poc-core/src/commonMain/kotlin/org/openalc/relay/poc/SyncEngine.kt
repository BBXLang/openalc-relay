package org.openalc.relay.poc

import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

sealed interface UploadResult {
    data class Accepted(val receipt: Receipt) : UploadResult
    data class Retry(val code: String, val retryAtMs: Long = 0) : UploadResult
    data class PermanentFailure(val code: String) : UploadResult
}

fun interface CollectorTransport {
    suspend fun upload(batch: Batch): UploadResult
}

enum class SyncOutcome { COMPLETE, RETRY }

class SyncEngine(
    private val outbox: OutboxDao,
    private val transport: CollectorTransport,
    private val now: () -> Long,
) {
    private val mutex = Mutex()

    suspend fun synchronize(maxBatches: Int = 4): SyncOutcome = mutex.withLock {
        require(maxBatches > 0)
        val installation = checkNotNull(outbox.installationId())
        repeat(maxBatches) {
            val pending = outbox.due(now())
            if (pending.isEmpty()) return@withLock outcome()
            val ids = pending.map { it.eventId }
            val batch = Batch(installationId = installation, events = pending.map(StoredEvent::wire))
            outbox.recordAttempt(ids)
            when (val result = transport.upload(batch)) {
                is UploadResult.Retry -> {
                    outbox.defer(ids, result.code, result.retryAtMs)
                    return@withLock SyncOutcome.RETRY
                }
                is UploadResult.PermanentFailure -> outbox.reject(ids, result.code)
                is UploadResult.Accepted -> {
                    if (!validReceipt(batch, result.receipt)) {
                        outbox.defer(ids, "invalid_ack", 0)
                        return@withLock SyncOutcome.RETRY
                    }
                    outbox.applyAcknowledgements(result.receipt.acknowledgements)
                    if (result.receipt.acknowledgements.size != pending.size) {
                        // Missing ACKs stay pending. Do not immediately hammer the collector.
                        return@withLock SyncOutcome.RETRY
                    }
                }
            }
        }
        outcome()
    }

    private suspend fun outcome() = if (outbox.pendingCount() == 0) SyncOutcome.COMPLETE else SyncOutcome.RETRY

    private fun validReceipt(batch: Batch, receipt: Receipt): Boolean {
        if (receipt.version != 1 || receipt.installationId != batch.installationId || receipt.acknowledgements.isEmpty()) return false
        val ids = batch.events.map { it.eventId }.toSet()
        val returned = mutableSetOf<String>()
        return receipt.acknowledgements.all { ack ->
            ack.eventId in ids && returned.add(ack.eventId) && when (ack.status) {
                "persisted", "duplicate" -> ack.code == null
                "rejected" -> ack.code in setOf("invalid_event", "event_id_conflict")
                else -> false
            }
        }
    }
}
