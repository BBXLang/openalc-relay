package org.openalc.relay.poc.android.poc

import android.Manifest
import android.content.pm.PackageManager
import androidx.annotation.StringRes
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import java.util.UUID
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import org.openalc.relay.poc.QueueCounts
import org.openalc.relay.poc.StoredEvent
import org.openalc.relay.poc.android.R

/**
 * UI state for the synthetic verification screen. It only re-exposes the existing PoC
 * application logic (Room queue, WorkManager sync, synthetic fixtures); the PoC behaviour
 * itself is unchanged. Note: the PoC core keeps working exactly as before — this is only
 * the presentation layer that moved out of the former home screen.
 */
class PocUiState(
    private val application: org.openalc.relay.poc.android.PocApplication,
    permissionGranted: Boolean,
) {
    var counts by mutableStateOf(QueueCounts(0, 0, 0))
        private set
    var installationId by mutableStateOf<String?>(null)
        private set
    var paused by mutableStateOf(application.uploadsPaused)
        private set
    var fixture by mutableStateOf(UUID.randomUUID().toString())
        private set
    var permission by mutableStateOf(permissionGranted)
        private set

    /** Read-only Flow of recent synthetic events from the existing Room queue. */
    val recentEvents: Flow<List<StoredEvent>> = application.outbox.observeRecent()

    /** Existing PoC diagnostic codes; mapped to strings via [diagnosticMessage]. */
    val diagnostic: StateFlow<String> = application.diagnostic

    fun refreshPermission(context: android.content.Context) {
        permission = context.checkSelfPermission(Manifest.permission.RECEIVE_SMS) ==
            PackageManager.PERMISSION_GRANTED
    }

    fun pauseUploads(paused: Boolean) {
        try {
            application.pauseUploads(paused)
            this.paused = paused
        } catch (error: CancellationException) {
            throw error
        } catch (_: Exception) {
            application.diagnostic.value = "pause_control_failed"
        }
    }

    fun newFixture() {
        fixture = UUID.randomUUID().toString()
    }

    fun persistFixture() {
        val id = fixture
        application.scope.launch {
            try {
                application.captureFixture(id, "injected")
            } catch (error: CancellationException) {
                throw error
            } catch (_: Exception) {
                application.diagnostic.value = "capture_failed"
            }
        }
    }

    fun requestSync() {
        application.scope.launch { application.syncOnUserRequest() }
    }

    suspend fun observeIdentityAndCounts() {
        try {
            installationId = application.initializeIdentity()
            application.outbox.observeCounts().collect { counts = it }
        } catch (error: CancellationException) {
            throw error
        } catch (_: Exception) {
            application.diagnostic.value = "queue_read_failed"
        }
    }

    companion object {
        @StringRes
        fun diagnosticMessage(code: String): Int = when (code) {
            "idle" -> R.string.status_idle
            "startup_failed" -> R.string.status_startup_failed
            "queue_read_failed" -> R.string.status_queue_read_failed
            "pause_control_failed" -> R.string.status_pause_control_failed
            "injected_persisted" -> R.string.status_injected_persisted
            "injected_duplicate" -> R.string.status_injected_duplicate
            "sms_broadcast_persisted" -> R.string.status_sms_persisted
            "sms_broadcast_duplicate" -> R.string.status_sms_duplicate
            "capture_failed" -> R.string.status_capture_failed
            "capture_interrupted" -> R.string.status_capture_interrupted
            "sync_complete" -> R.string.status_sync_complete
            "sync_paused" -> R.string.status_sync_paused
            "sync_waiting_retry" -> R.string.status_sync_waiting_retry
            "sync_failed" -> R.string.status_sync_failed
            else -> R.string.status_unknown
        }
    }
}
