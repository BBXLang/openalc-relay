package org.openalc.relay.poc.android

import android.app.Application
import androidx.room.Room
import androidx.sqlite.driver.bundled.BundledSQLiteDriver
import java.util.UUID
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.flow.MutableStateFlow
import org.openalc.relay.poc.LoopbackTransport
import org.openalc.relay.poc.PocDatabase
import org.openalc.relay.poc.SyncEngine
import org.openalc.relay.poc.SyncOutcome

class PocApplication : Application() {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    val diagnostic = MutableStateFlow("idle")
    val database: PocDatabase by lazy {
        Room.databaseBuilder<PocDatabase>(this, getDatabasePath("synthetic-outbox.db").absolutePath)
            .setDriver(BundledSQLiteDriver())
            .setQueryCoroutineContext(Dispatchers.IO)
            .build()
    }
    val outbox get() = database.outbox()
    val syncEngine by lazy { SyncEngine(outbox, LoopbackTransport(), System::currentTimeMillis) }
    private val preferences by lazy { getSharedPreferences("poc-controls", MODE_PRIVATE) }
    val uploadsPaused get() = preferences.getBoolean("uploads-paused", false)

    override fun onCreate() {
        super.onCreate()
        scope.launch {
            try {
                initializeIdentity()
                SyncScheduler.installRepair(this@PocApplication)
                SyncScheduler.request(this@PocApplication)
            } catch (error: CancellationException) {
                throw error
            } catch (_: Exception) {
                diagnostic.value = "startup_failed"
            }
        }
    }

    suspend fun initializeIdentity(): String = outbox.initialize(UUID.randomUUID().toString())

    fun pauseUploads(paused: Boolean) {
        // This control prevents new sync runs. An already running bounded sync may finish.
        check(preferences.edit().putBoolean("uploads-paused", paused).commit())
        if (!paused) SyncScheduler.request(this)
    }

    suspend fun syncOnUserRequest() {
        if (uploadsPaused) {
            diagnostic.value = "sync_paused"
            return
        }
        try {
            initializeIdentity()
            when (syncEngine.synchronize()) {
                SyncOutcome.COMPLETE -> diagnostic.value = "sync_complete"
                SyncOutcome.RETRY -> {
                    diagnostic.value = "sync_waiting_retry"
                    SyncScheduler.request(this)
                }
            }
        } catch (error: CancellationException) {
            throw error
        } catch (_: Exception) {
            diagnostic.value = "sync_failed"
            SyncScheduler.request(this)
        }
    }

    suspend fun captureFixture(id: String, source: String, subscriptionId: Int? = null) {
        initializeIdentity()
        val inserted = outbox.capture(id, source, System.currentTimeMillis(), subscriptionId)
        diagnostic.value = if (inserted) "${source}_persisted" else "${source}_duplicate"
        // Enqueue only after the durable transaction. Repair handles a crash before this call.
        SyncScheduler.request(this)
    }
}
