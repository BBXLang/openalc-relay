package org.openalc.relay.poc

import java.io.ByteArrayOutputStream
import java.io.IOException
import java.net.HttpURLConnection
import java.net.Proxy
import java.net.URI
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString

// Only adb reverse / JVM-local HTTP is permitted. This is not the production HTTPS transport.
class LoopbackTransport(
    endpoint: String = "http://127.0.0.1:8787/poc/v1/events",
    private val now: () -> Long = System::currentTimeMillis,
) : CollectorTransport {
    private val uri = URI(endpoint).also {
        require(it.scheme == "http" && it.host == "127.0.0.1" && it.port in 1..65535)
        require(it.rawPath == "/poc/v1/events" && it.rawQuery == null && it.rawFragment == null && it.userInfo == null)
    }

    override suspend fun upload(batch: Batch): UploadResult = withContext(Dispatchers.IO) {
        val connection = uri.toURL().openConnection(Proxy.NO_PROXY) as HttpURLConnection
        try {
            connection.requestMethod = "POST"
            connection.instanceFollowRedirects = false
            connection.connectTimeout = 5_000
            connection.readTimeout = 5_000
            connection.setRequestProperty("Content-Type", "application/json")
            connection.setRequestProperty("Accept", "application/json")
            connection.doOutput = true
            val body = protocolJson.encodeToString(batch).toByteArray(Charsets.UTF_8)
            connection.setFixedLengthStreamingMode(body.size)
            connection.outputStream.use { it.write(body) }
            when (val status = connection.responseCode) {
                200 -> {
                    val bytes = ByteArrayOutputStream()
                    connection.inputStream.use { input ->
                        val buffer = ByteArray(4096)
                        while (true) {
                            val length = input.read(buffer)
                            if (length < 0) break
                            if (bytes.size() + length > 64 * 1024) return@withContext UploadResult.Retry("invalid_ack")
                            bytes.write(buffer, 0, length)
                        }
                    }
                    try {
                        UploadResult.Accepted(protocolJson.decodeFromString<Receipt>(bytes.toString("UTF-8")))
                    } catch (_: SerializationException) {
                        UploadResult.Retry("invalid_ack")
                    } catch (_: IllegalArgumentException) {
                        UploadResult.Retry("invalid_ack")
                    }
                }
                408, 425, 429 -> UploadResult.Retry("http_retryable", retryAt(connection.getHeaderField("Retry-After")))
                in 500..599 -> UploadResult.Retry("http_retryable", retryAt(connection.getHeaderField("Retry-After")))
                in 300..399 -> UploadResult.PermanentFailure("redirect_blocked")
                else -> UploadResult.PermanentFailure(if (status in 400..499) "request_rejected" else "unexpected_http_status")
            }
        } catch (_: IOException) {
            UploadResult.Retry("network_unavailable")
        } finally {
            connection.disconnect()
        }
    }

    private fun retryAt(header: String?): Long {
        if (header == null) return 0
        val current = now()
        val seconds = header.toLongOrNull()
        val target = if (seconds != null) {
            current + seconds.coerceIn(0, 86400) * 1000
        } else {
            try {
                ZonedDateTime.parse(header, DateTimeFormatter.RFC_1123_DATE_TIME).toInstant().toEpochMilli()
            } catch (_: RuntimeException) {
                return 0
            }
        }
        return target.coerceIn(current, current + 86_400_000)
    }
}
