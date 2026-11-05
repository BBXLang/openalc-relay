package org.openalc.relay.poc

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

const val FIXTURE = "openalc-relay-synthetic-v1"
const val SMS_PREFIX = "OPENALC_RELAY_POC:v1:"
const val MAX_BATCH = 25
const val MAX_PENDING = 10_000
const val PENDING = "pending"
const val SYNCED = "synced"
const val REJECTED = "rejected"

val protocolJson = Json {
    encodeDefaults = true
    ignoreUnknownKeys = false
}

private val uuidPattern = Regex("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
fun isCanonicalUuid(value: String): Boolean = uuidPattern.matches(value)

// This is an allowlist, not a production SMS parser. Non-fixture bodies never reach storage.
fun syntheticFixtureId(parts: List<String?>): String? {
    if (parts.isEmpty() || parts.size > 8 || parts.any { it == null }) return null
    if (parts.sumOf { it!!.length } != SMS_PREFIX.length + 36) return null
    val body = parts.joinToString("") { it!! }
    if (!body.startsWith(SMS_PREFIX)) return null
    return body.removePrefix(SMS_PREFIX).takeIf(::isCanonicalUuid)
}

@Serializable
data class WireEvent(
    @SerialName("event_id") val eventId: String,
    @SerialName("fixture_id") val fixtureId: String,
    val source: String,
    @SerialName("received_at_ms") val receivedAtMs: Long,
    @SerialName("subscription_id") val subscriptionId: Int?,
)

@Serializable
data class Batch(
    val version: Int = 1,
    val fixture: String = FIXTURE,
    @SerialName("installation_id") val installationId: String,
    val events: List<WireEvent>,
)

@Serializable
data class Acknowledgement(
    @SerialName("event_id") val eventId: String,
    val status: String,
    val code: String? = null,
)

@Serializable
data class Receipt(
    val version: Int,
    @SerialName("installation_id") val installationId: String,
    val acknowledgements: List<Acknowledgement>,
)
