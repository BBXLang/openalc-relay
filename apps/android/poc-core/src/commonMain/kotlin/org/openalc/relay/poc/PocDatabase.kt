package org.openalc.relay.poc

import androidx.room.ConstructedBy
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.RoomDatabase
import androidx.room.RoomDatabaseConstructor
import androidx.room.Transaction
import kotlinx.coroutines.flow.Flow

@Entity(tableName = "installation")
data class Installation(@PrimaryKey val singleton: Int = 1, val installationId: String)

@Entity(tableName = "outbox_events")
data class StoredEvent(
    @PrimaryKey val eventId: String,
    val fixtureId: String,
    val source: String,
    val receivedAtMs: Long,
    val subscriptionId: Int?,
    val state: String = PENDING,
    val attempts: Int = 0,
    val nextAttemptAtMs: Long = 0,
    val lastError: String? = null,
) {
    fun wire() = WireEvent(eventId, fixtureId, source, receivedAtMs, subscriptionId)
}

data class QueueCounts(val pending: Int, val synced: Int, val rejected: Int)

@Dao
abstract class OutboxDao {
    @Insert(onConflict = OnConflictStrategy.IGNORE)
    protected abstract suspend fun insertInstallation(value: Installation)

    @Query("SELECT installationId FROM installation WHERE singleton = 1")
    abstract suspend fun installationId(): String?

    @Transaction
    open suspend fun initialize(proposed: String): String {
        require(isCanonicalUuid(proposed))
        insertInstallation(Installation(installationId = proposed))
        return checkNotNull(installationId())
    }

    @Insert(onConflict = OnConflictStrategy.IGNORE)
    protected abstract suspend fun insertEvent(value: StoredEvent): Long

    @Query("SELECT * FROM outbox_events WHERE eventId = :id")
    abstract suspend fun find(id: String): StoredEvent?

    @Query("SELECT count(*) FROM outbox_events WHERE state = 'pending'")
    abstract suspend fun pendingCount(): Int

    @Transaction
    open suspend fun capture(fixtureId: String, source: String, receivedAtMs: Long, subscriptionId: Int?): Boolean {
        require(isCanonicalUuid(fixtureId))
        require(source == "injected" || source == "sms_broadcast")
        require(receivedAtMs in 1 until 4102444800000L)
        require(subscriptionId == null || subscriptionId >= 0)
        check(installationId() != null)
        // Repeated reception of this synthetic fixture preserves its first timestamp and payload.
        if (find(fixtureId) != null) return false
        check(pendingCount() < MAX_PENDING) { "synthetic_queue_full" }
        return insertEvent(StoredEvent(fixtureId, fixtureId, source, receivedAtMs, subscriptionId)) != -1L
    }

    @Query("SELECT * FROM outbox_events WHERE state = 'pending' AND nextAttemptAtMs <= :now ORDER BY receivedAtMs, eventId LIMIT :limit")
    abstract suspend fun due(now: Long, limit: Int = MAX_BATCH): List<StoredEvent>

    @Query("UPDATE outbox_events SET attempts = attempts + 1 WHERE eventId IN (:ids) AND state = 'pending'")
    abstract suspend fun recordAttempt(ids: List<String>)

    @Query("UPDATE outbox_events SET lastError = :code, nextAttemptAtMs = :retryAt WHERE eventId IN (:ids) AND state = 'pending'")
    abstract suspend fun defer(ids: List<String>, code: String, retryAt: Long)

    @Query("UPDATE outbox_events SET state = 'synced', lastError = NULL, nextAttemptAtMs = 0 WHERE eventId = :id AND state = 'pending'")
    protected abstract suspend fun markSynced(id: String)

    @Query("UPDATE outbox_events SET state = 'rejected', lastError = :code WHERE eventId = :id AND state = 'pending'")
    protected abstract suspend fun markRejected(id: String, code: String)

    @Transaction
    open suspend fun applyAcknowledgements(acks: List<Acknowledgement>) {
        for (ack in acks) {
            when (ack.status) {
                "persisted", "duplicate" -> markSynced(ack.eventId)
                "rejected" -> markRejected(ack.eventId, checkNotNull(ack.code))
                else -> error("invalid_ack")
            }
        }
    }

    @Transaction
    open suspend fun reject(ids: List<String>, code: String) {
        for (id in ids) markRejected(id, code)
    }

    @Query("SELECT COALESCE(SUM(CASE WHEN state = 'pending' THEN 1 ELSE 0 END), 0) AS pending, COALESCE(SUM(CASE WHEN state = 'synced' THEN 1 ELSE 0 END), 0) AS synced, COALESCE(SUM(CASE WHEN state = 'rejected' THEN 1 ELSE 0 END), 0) AS rejected FROM outbox_events")
    abstract fun observeCounts(): Flow<QueueCounts>

    @Query("SELECT * FROM outbox_events ORDER BY receivedAtMs DESC, eventId DESC LIMIT 50")
    abstract fun observeRecent(): Flow<List<StoredEvent>>
}

@Database(entities = [Installation::class, StoredEvent::class], version = 1, exportSchema = true)
@ConstructedBy(PocDatabaseConstructor::class)
abstract class PocDatabase : RoomDatabase() {
    abstract fun outbox(): OutboxDao
}

@Suppress("NO_ACTUAL_FOR_EXPECT")
expect object PocDatabaseConstructor : RoomDatabaseConstructor<PocDatabase> {
    override fun initialize(): PocDatabase
}
