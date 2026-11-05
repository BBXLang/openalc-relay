package org.openalc.relay.poc.android

import android.Manifest
import android.content.pm.PackageManager
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.enableEdgeToEdge
import androidx.activity.compose.setContent
import androidx.annotation.StringRes
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.res.stringResource
import java.util.UUID
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import org.openalc.relay.poc.QueueCounts
import org.openalc.relay.poc.StoredEvent

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.light(android.graphics.Color.TRANSPARENT, android.graphics.Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.light(android.graphics.Color.TRANSPARENT, android.graphics.Color.TRANSPARENT),
        )
        val application = application as PocApplication
        setContent {
            var counts by remember { mutableStateOf(QueueCounts(0, 0, 0)) }
            var installationId by remember { mutableStateOf<String?>(null) }
            var paused by remember { mutableStateOf(application.uploadsPaused) }
            var fixture by remember { mutableStateOf(UUID.randomUUID().toString()) }
            var permission by remember {
                mutableStateOf(checkSelfPermission(Manifest.permission.RECEIVE_SMS) == PackageManager.PERMISSION_GRANTED)
            }
            val recent by application.outbox.observeRecent().collectAsState(initial = emptyList<StoredEvent>())
            val diagnostic by application.diagnostic.collectAsState()
            LaunchedEffect(Unit) {
                try {
                    installationId = application.initializeIdentity()
                    application.outbox.observeCounts().collect { counts = it }
                } catch (error: CancellationException) {
                    throw error
                } catch (_: Exception) {
                    application.diagnostic.value = "queue_read_failed"
                }
            }
            RelayInboxScreen(
                counts = counts,
                recent = recent,
                installationId = installationId,
                status = stringResource(diagnosticMessage(diagnostic)),
                permission = permission,
                paused = paused,
                fixture = fixture,
                onPauseChanged = {
                    try {
                        application.pauseUploads(it)
                        paused = it
                    } catch (_: Exception) {
                        application.diagnostic.value = "pause_control_failed"
                    }
                },
                onNewFixture = { fixture = UUID.randomUUID().toString() },
                onPersistFixture = {
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
                },
                onRequestSync = { application.scope.launch { application.syncOnUserRequest() } },
            )
        }
    }
}

@StringRes
private fun diagnosticMessage(code: String): Int = when (code) {
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
