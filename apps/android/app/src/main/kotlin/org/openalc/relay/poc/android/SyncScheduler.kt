package org.openalc.relay.poc.android

import android.content.Context
import android.os.Build
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.OutOfQuotaPolicy
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import java.util.concurrent.TimeUnit

object SyncScheduler {
    private val network = Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build()

    fun request(context: Context) {
        val request = OneTimeWorkRequestBuilder<PocSyncWorker>()
            .setConstraints(network)
            .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
        // Pre-Android 12 expedited jobs may require a foreground service. This PoC starts none.
        if (Build.VERSION.SDK_INT >= 31) {
            request.setExpedited(OutOfQuotaPolicy.RUN_AS_NON_EXPEDITED_WORK_REQUEST)
        }
        // APPEND_OR_REPLACE preserves an arrival racing a currently finishing worker.
        WorkManager.getInstance(context).enqueueUniqueWork(
            "synthetic-outbox-upload", ExistingWorkPolicy.APPEND_OR_REPLACE, request.build(),
        )
    }

    fun installRepair(context: Context) {
        val request = PeriodicWorkRequestBuilder<PocRepairWorker>(15, TimeUnit.MINUTES).build()
        WorkManager.getInstance(context).enqueueUniquePeriodicWork(
            "synthetic-outbox-repair", ExistingPeriodicWorkPolicy.KEEP, request,
        )
    }
}
