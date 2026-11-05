package org.openalc.relay.poc.android.components

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/**
 * SMS conversation card per plan section 2.2: source device, SIM display name and confirmed local
 * number, sender, snippet, time and send/receive state. When the number cannot be read reliably the
 * card shows the "number unconfirmed" wording instead of guessing a number.
 */
data class SmsCardData(
    val deviceName: String?,
    val simLabel: String?,
    /** Confirmed local number; null renders the unconfirmed wording. */
    val confirmedLocalNumber: String?,
    val sender: String,
    val snippet: String,
    val timeText: String,
    val statusText: String,
)

@Composable
fun SmsCard(
    data: SmsCardData,
    modifier: Modifier = Modifier,
    isDemo: Boolean = false,
    onClick: (() -> Unit)? = null,
) {
    val palette = LocalRelayPalette.current
    CardSurface(modifier = modifier, isDemo = isDemo, onClick = onClick) {
        if (isDemo) {
            DemoBadge()
            Spacer(Modifier.height(8.dp))
        }
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text(
                data.sender,
                modifier = Modifier.weight(1f),
                fontSize = 14.sp,
                fontWeight = FontWeight.SemiBold,
                color = palette.ink,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(data.timeText, fontSize = 10.sp, color = palette.muted)
        }
        Spacer(Modifier.height(4.dp))
        val numberText = data.confirmedLocalNumber ?: stringResource(R.string.number_unconfirmed)
        val sourceLine = buildString {
            data.deviceName?.let { append(it) }
            if (data.simLabel != null) {
                if (isNotEmpty()) append(" · ")
                append(data.simLabel)
            }
            if (isNotEmpty()) append(" · ")
            append(numberText)
        }
        Text(sourceLine, fontSize = 11.sp, color = palette.muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Spacer(Modifier.height(6.dp))
        Text(
            data.snippet,
            fontSize = 13.sp,
            lineHeight = 19.sp,
            color = palette.ink,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
        Spacer(Modifier.height(6.dp))
        Text(data.statusText, fontSize = 11.sp, color = palette.muted)
    }
}

/**
 * Mail card per plan section 2.2: receiving account, sender, subject, snippet, time,
 * attachment marker and unread state.
 */
data class MailCardData(
    val accountAddress: String,
    val sender: String,
    val subject: String,
    val snippet: String,
    val timeText: String,
    val hasAttachment: Boolean,
    val unread: Boolean,
)

@Composable
fun MailCard(
    data: MailCardData,
    modifier: Modifier = Modifier,
    isDemo: Boolean = false,
    onClick: (() -> Unit)? = null,
) {
    val palette = LocalRelayPalette.current
    CardSurface(modifier = modifier, isDemo = isDemo, onClick = onClick) {
        if (isDemo) {
            DemoBadge()
            Spacer(Modifier.height(8.dp))
        }
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            if (data.unread) {
                Box(
                    Modifier
                        .size(8.dp)
                        .background(palette.yellow, CircleShape),
                )
                Spacer(Modifier.width(8.dp))
            }
            Text(
                data.sender,
                modifier = Modifier.weight(1f),
                fontSize = 14.sp,
                fontWeight = FontWeight.SemiBold,
                color = palette.ink,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(data.timeText, fontSize = 10.sp, color = palette.muted)
        }
        Spacer(Modifier.height(4.dp))
        Text(
            data.subject,
            fontSize = 13.sp,
            color = palette.ink,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Spacer(Modifier.height(4.dp))
        Text(
            data.snippet,
            fontSize = 12.sp,
            lineHeight = 17.sp,
            color = palette.muted,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
        Spacer(Modifier.height(6.dp))
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                data.accountAddress,
                modifier = Modifier.weight(1f),
                fontSize = 11.sp,
                color = palette.muted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (data.hasAttachment) {
                Text(stringResource(R.string.attachment_mark), fontSize = 11.sp, color = palette.muted)
            }
        }
    }
}

@Composable
private fun CardSurface(
    modifier: Modifier,
    isDemo: Boolean,
    onClick: (() -> Unit)?,
    content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit,
) {
    val palette = LocalRelayPalette.current
    val clickableModifier = if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier
    Surface(
        modifier = modifier
            .fillMaxWidth()
            .then(clickableModifier),
        shape = RoundedCornerShape(18.dp),
        color = palette.surface,
        border = androidx.compose.foundation.BorderStroke(1.dp, palette.segment),
    ) {
        Column(Modifier.padding(14.dp), content = content)
    }
}

@Composable
private fun DemoBadge() {
    val palette = LocalRelayPalette.current
    Box(
        Modifier
            .background(palette.yellow, RoundedCornerShape(50))
            .padding(horizontal = 8.dp, vertical = 3.dp),
    ) {
        Text(
            stringResource(R.string.demo_badge),
            fontSize = 10.sp,
            color = palette.contentOnYellow,
        )
    }
}
