package org.openalc.relay.poc.android

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import kotlinx.coroutines.CancellationException
import org.openalc.relay.poc.SyncOutcome

class PocSyncWorker(context: Context, parameters: WorkerParameters) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result {
        val application = applicationContext as PocApplication
        if (application.uploadsPaused) return Result.success()
        return try {
            application.initializeIdentity()
            when (application.syncEngine.synchronize()) {
                SyncOutcome.COMPLETE -> {
                    application.diagnostic.value = "sync_complete"
                    Result.success()
                }
                SyncOutcome.RETRY -> {
                    application.diagnostic.value = "sync_waiting_retry"
                    Result.retry()
                }
            }
        } catch (error: CancellationException) {
            throw error
        } catch (_: Exception) {
            application.diagnostic.value = "sync_failed"
            Result.retry()
        }
    }
}

class PocRepairWorker(context: Context, parameters: WorkerParameters) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result {
        val application = applicationContext as PocApplication
        return try {
            application.initializeIdentity()
            if (!application.uploadsPaused && application.outbox.pendingCount() > 0) {
                SyncScheduler.request(application)
            }
            Result.success()
        } catch (error: CancellationException) {
            throw error
        } catch (_: Exception) {
            Result.retry()
        }
    }
}
