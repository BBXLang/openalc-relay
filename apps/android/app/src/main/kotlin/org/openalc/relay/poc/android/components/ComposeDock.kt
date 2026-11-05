package org.openalc.relay.poc.android.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/**
 * Bottom floating compose dock (plan sections 2.1 and 2.3):
 * - SMS page: small recipient number input, left device/SIM picker button showing the current source;
 * - Mail page: recipient address input, left sender account picker button;
 * - main input bar for the body; send control is disabled with the honest reason until the formal
 *   transport is connected.
 *
 * Drafts live in SharedPreferences with the channel in the key (see RelayAppState).
 * 真实正文草稿在正式链路开放时须加密保存；当前仅保存正式链路未接通时的输入草稿。
 */
@Composable
fun ComposeDock(
    recipientHint: String,
    recipient: String,
    onRecipientChange: (String) -> Unit,
    bodyHint: String,
    body: String,
    onBodyChange: (String) -> Unit,
    sourceSummary: String?,
    isSmsChannel: Boolean,
    onPickSource: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = LocalRelayPalette.current
    val sendReason = stringResource(R.string.compose_send_disabled_reason)
    Surface(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 14.dp)
            .shadow(12.dp, RoundedCornerShape(32.dp)),
        color = palette.surface,
        shape = RoundedCornerShape(32.dp),
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                BasicTextField(
                    value = recipient,
                    onValueChange = onRecipientChange,
                    modifier = Modifier.weight(1f).padding(horizontal = 8.dp),
                    singleLine = true,
                    textStyle = TextStyle(color = palette.ink, fontSize = 13.sp),
                    cursorBrush = SolidColor(palette.ink),
                    decorationBox = { inner ->
                        Box {
                            if (recipient.isEmpty()) {
                                Text(recipientHint, color = palette.muted, fontSize = 13.sp)
                            }
                            inner()
                        }
                    },
                )
            }
            Spacer(Modifier.height(6.dp))
            Row(
                modifier = Modifier.fillMaxWidth().height(52.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .background(palette.segment, CircleShape)
                        .clickable(onClick = onPickSource),
                    contentAlignment = Alignment.Center,
                ) {
                    if (isSmsChannel) DeviceSimGlyph(palette.ink) else EnvelopeGlyph(palette.ink)
                }
                Column(Modifier.width(74.dp).padding(start = 8.dp)) {
                    Text(
                        stringResource(if (isSmsChannel) R.string.compose_source_sms else R.string.compose_source_mail),
                        fontSize = 9.sp,
                        color = palette.muted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Text(
                        sourceSummary ?: stringResource(R.string.compose_source_unselected),
                        fontSize = 10.sp,
                        color = palette.ink,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                BasicTextField(
                    value = body,
                    onValueChange = onBodyChange,
                    modifier = Modifier.weight(1f).padding(horizontal = 8.dp),
                    maxLines = 4,
                    textStyle = TextStyle(color = palette.ink, fontSize = 15.sp),
                    cursorBrush = SolidColor(palette.ink),
                    decorationBox = { inner ->
                        Box {
                            if (body.isEmpty()) {
                                Text(bodyHint, color = palette.muted, fontSize = 15.sp)
                            }
                            inner()
                        }
                    },
                )
                Button(
                    onClick = {},
                    enabled = false,
                    modifier = Modifier
                        .size(44.dp)
                        .semantics { contentDescription = sendReason },
                    shape = CircleShape,
                    colors = ButtonDefaults.buttonColors(
                        containerColor = palette.yellow,
                        contentColor = palette.contentOnYellow,
                        disabledContainerColor = palette.yellow.copy(alpha = 0.35f),
                        disabledContentColor = palette.contentOnYellow.copy(alpha = 0.6f),
                    ),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(0.dp),
                ) {
                    SendGlyph(palette.contentOnYellow)
                }
            }
            Spacer(Modifier.height(4.dp))
            Text(
                stringResource(R.string.compose_send_disabled_reason),
                modifier = Modifier.padding(start = 10.dp),
                fontSize = 10.sp,
                color = palette.muted,
            )
        }
    }
}

/** Device + SIM glyph for the SMS source picker. */
@Composable
private fun DeviceSimGlyph(color: Color) {
    Canvas(Modifier.fillMaxSize()) {
        val stroke = Stroke(width = 1.8.dp.toPx(), cap = StrokeCap.Round)
        val phone = Path().apply {
            addOval(androidx.compose.ui.geometry.Rect(0.24f * size.width, 0.22f * size.height, 0.52f * size.width, 0.62f * size.height))
        }
        drawPath(phone, color, style = stroke)
        val sim = Path().apply {
            moveTo(0.52f * size.width, 0.38f * size.height)
            lineTo(0.78f * size.width, 0.38f * size.height)
            lineTo(0.78f * size.width, 0.78f * size.height)
            lineTo(0.52f * size.width, 0.78f * size.height)
            close()
        }
        drawPath(sim, color, style = stroke)
    }
}

/** Envelope glyph for the mail sender account picker. */
@Composable
private fun EnvelopeGlyph(color: Color) {
    Canvas(Modifier.fillMaxSize()) {
        val stroke = Stroke(width = 1.8.dp.toPx(), cap = StrokeCap.Round)
        val rect = androidx.compose.ui.geometry.Rect(0.2f * size.width, 0.3f * size.height, 0.8f * size.width, 0.7f * size.height)
        drawRect(color, topLeft = rect.topLeft, size = rect.size, style = stroke)
        val flap = Path().apply {
            moveTo(0.2f * size.width, 0.34f * size.height)
            lineTo(0.5f * size.width, 0.55f * size.height)
            lineTo(0.8f * size.width, 0.34f * size.height)
        }
        drawPath(flap, color, style = stroke)
    }
}

/** Send glyph (paper plane) for the disabled send button. */
@Composable
private fun SendGlyph(color: Color) {
    Canvas(Modifier.fillMaxSize()) {
        val plane = Path().apply {
            moveTo(0.22f * size.width, 0.5f * size.height)
            lineTo(0.8f * size.width, 0.26f * size.height)
            lineTo(0.56f * size.width, 0.78f * size.height)
            lineTo(0.46f * size.width, 0.55f * size.height)
            close()
        }
        drawPath(plane, color)
    }
}
