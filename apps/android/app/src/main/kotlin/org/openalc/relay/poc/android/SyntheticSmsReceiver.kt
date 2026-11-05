package org.openalc.relay.poc.android

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.provider.Telephony
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeout
import org.openalc.relay.poc.syntheticFixtureId

class SyntheticSmsReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Telephony.Sms.Intents.SMS_RECEIVED_ACTION) return
        val application = context.applicationContext as PocApplication
        val pending = goAsync()
        application.scope.launch {
            try {
                withTimeout(7_000) {
                    // Numbers and original PDU/body data are never retained, logged, or uploaded.
                    val parts = Telephony.Sms.Intents.getMessagesFromIntent(intent)
                        .map { it.messageBody }
                    val id = syntheticFixtureId(parts) ?: return@withTimeout
                    // SMS_RECEIVED subscription extras vary by platform; never invent a SIM mapping.
                    application.captureFixture(id, "sms_broadcast")
                }
            } catch (error: CancellationException) {
                application.diagnostic.value = "capture_interrupted"
                throw error
            } catch (_: Exception) {
                application.diagnostic.value = "capture_failed"
            } finally {
                pending.finish()
            }
        }
    }
}
