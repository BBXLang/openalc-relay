package org.openalc.relay.poc

import com.sun.net.httpserver.HttpServer
import java.net.InetSocketAddress
import java.util.UUID
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.encodeToString
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertIs

class LoopbackTransportTest {
    private val eventId = "00000000-0000-4000-8000-000000000002"
    private fun batch() = Batch(installationId = UUID.randomUUID().toString(), events = listOf(WireEvent(eventId, eventId, "injected", 1793577600000, null)))

    private fun server(status: Int, body: (Batch) -> String, header: Pair<String, String>? = null, test: suspend (String) -> Unit) = runBlocking {
        val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)
        server.createContext("/poc/v1/events") { exchange ->
            val received = protocolJson.decodeFromString<Batch>(exchange.requestBody.reader().readText())
            val bytes = body(received).toByteArray()
            header?.let { exchange.responseHeaders.add(it.first, it.second) }
            exchange.sendResponseHeaders(status, bytes.size.toLong())
            exchange.responseBody.use { it.write(bytes) }
        }
        server.start()
        try {
            test("http://127.0.0.1:${server.address.port}/poc/v1/events")
        } finally {
            server.stop(0)
        }
    }

    @Test
    fun actualHttpRoundTripUsesTypedSyntheticProtocol() = server(200, { batch ->
        protocolJson.encodeToString(Receipt(1, batch.installationId, listOf(Acknowledgement(eventId, "persisted"))))
    }) { endpoint ->
        val result = assertIs<UploadResult.Accepted>(LoopbackTransport(endpoint).upload(batch()))
        assertEquals("persisted", result.receipt.acknowledgements.single().status)
    }

    @Test
    fun rateLimitPersistsServerRetryDeadline() = server(429, { "{}" }, "Retry-After" to "120") { endpoint ->
        val result = assertIs<UploadResult.Retry>(LoopbackTransport(endpoint) { 1000 }.upload(batch()))
        assertEquals(121000L, result.retryAtMs)
    }

    @Test
    fun redirectsAreNotFollowed() = server(302, { "{}" }, "Location" to "https://example.invalid/") { endpoint ->
        assertEquals(UploadResult.PermanentFailure("redirect_blocked"), LoopbackTransport(endpoint).upload(batch()))
    }

    @Test
    fun malformedAndOversizedResponsesStayRetryable() {
        for (body in listOf("not-json", "x".repeat(65537))) {
            server(200, { body }) { endpoint ->
                assertEquals(UploadResult.Retry("invalid_ack"), LoopbackTransport(endpoint).upload(batch()))
            }
        }
    }

    @Test
    fun productionAndAmbiguousEndpointsAreRejected() {
        for (endpoint in listOf("http://example.invalid:8787/poc/v1/events", "http://127.0.0.1:8787/poc/v1/events?body=x", "http://user@127.0.0.1:8787/poc/v1/events", "http://127.0.0.1:8787/poc/v1/events#x")) {
            assertFailsWith<IllegalArgumentException> { LoopbackTransport(endpoint) }
        }
    }
}
