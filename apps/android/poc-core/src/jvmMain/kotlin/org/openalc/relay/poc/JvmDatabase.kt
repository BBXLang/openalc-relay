package org.openalc.relay.poc

import androidx.room.Room
import androidx.sqlite.driver.bundled.BundledSQLiteDriver
import kotlinx.coroutines.Dispatchers

fun openJvmDatabase(path: String): PocDatabase = Room.databaseBuilder<PocDatabase>(path)
    .setDriver(BundledSQLiteDriver())
    .setQueryCoroutineContext(Dispatchers.IO)
    .build()
