package org.openalc.relay.poc.android

import android.content.Context
import androidx.room.Room
import androidx.sqlite.driver.bundled.BundledSQLiteDriver
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import java.io.File
import java.util.UUID
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.openalc.relay.poc.PENDING
import org.openalc.relay.poc.PocDatabase

@RunWith(AndroidJUnit4::class)
class AndroidPersistenceTest {
    @Test
    fun pendingFixtureSurvivesDatabaseReopen() = runBlocking {
        val context = ApplicationProvider.getApplicationContext<Context>()
        val path = File(context.cacheDir, "test-${UUID.randomUUID()}.db")
        fun open() = Room.databaseBuilder<PocDatabase>(context, path.absolutePath)
            .setDriver(BundledSQLiteDriver()).setQueryCoroutineContext(Dispatchers.IO).build()
        val id = UUID.randomUUID().toString()
        try {
            open().let { database ->
                database.outbox().initialize(UUID.randomUUID().toString())
                database.outbox().capture(id, "injected", 1793577600000, null)
                database.close()
            }
            open().let { database ->
                assertEquals(PENDING, database.outbox().find(id)?.state)
                database.close()
            }
        } finally {
            path.delete()
            File(path.path + "-wal").delete()
            File(path.path + "-shm").delete()
        }
    }
}
