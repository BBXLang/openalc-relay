package org.openalc.relay.poc.android.screens

import android.text.format.DateFormat
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.flow.Flow
import org.openalc.relay.poc.SMS_PREFIX
import org.openalc.relay.poc.StoredEvent
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.components.DetailTopBar
import org.openalc.relay.poc.android.components.MailCard
import org.openalc.relay.poc.android.components.MailCardData
import org.openalc.relay.poc.android.components.SmsCard
import org.openalc.relay.poc.android.components.SmsCardData
import org.openalc.relay.poc.android.poc.PocUiState
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/**
 * Development-only entry holding every former PoC operation (persist/new fixture, sync, pause
 * uploads, all former drawer hints) plus sample cards explicitly marked as demo data. This screen
 * is deliberately separated from the formal home screen (plan section 2.1); sample cards appear
 * only here.
 */
@Composable
fun SyntheticVerificationScreen(
    poc: PocUiState,
    recentEvents: Flow<List<StoredEvent>>,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = LocalRelayPalette.current
    val diagnostic by poc.diagnostic.collectAsState()
    val recent by recentEvents.collectAsState(initial = emptyList<StoredEvent>())
    var eventQuery by remember { mutableStateOf("") }
    val visibleEvents = remember(recent, eventQuery) {
        if (eventQuery.isBlank()) {
            recent
        } else {
            recent.filter { it.eventId.contains(eventQuery.trim(), ignoreCase = true) }
        }
    }

    Column(
        modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        DetailTopBar(title = stringResource(R.string.synthetic_verification_title), onBack = onBack)
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(horizontal = 22.dp, vertical = 8.dp),
        ) {
            item {
                SyntheticPanel(
                    poc = poc,
                    status = stringResource(PocUiState.diagnosticMessage(diagnostic)),
                )
            }
            item {
                Column(Modifier.fillMaxWidth()) {
                    Text(
                        stringResource(R.string.synthetic_events_section),
                        modifier = Modifier.padding(top = 22.dp, bottom = 10.dp),
                        fontSize = 13.sp,
                        color = palette.muted,
                    )
                    BasicTextField(
                        value = eventQuery,
                        onValueChange = { eventQuery = it },
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(palette.surface, CircleShape)
                            .padding(horizontal = 16.dp, vertical = 12.dp),
                        singleLine = true,
                        textStyle = TextStyle(color = palette.ink, fontSize = 13.sp),
                        cursorBrush = SolidColor(palette.ink),
                        decorationBox = { inner ->
                            Box {
                                if (eventQuery.isEmpty()) {
                                    Text(
                                        stringResource(R.string.synthetic_events_filter_hint),
                                        color = palette.muted,
                                        fontSize = 13.sp,
                                    )
                                }
                                inner()
                            }
                        },
                    )
                }
            }
            if (visibleEvents.isEmpty()) {
                item {
                    Text(
                        stringResource(R.string.synthetic_events_no_match),
                        modifier = Modifier.padding(top = 18.dp),
                        fontSize = 12.sp,
                        color = palette.muted,
                    )
                }
            } else {
                items(visibleEvents.size) { index ->
                    SyntheticEventRow(visibleEvents[index])
                }
            }
            item {
                DemoCardsSection()
            }
        }
    }
}

@Composable
private fun SyntheticPanel(poc: PocUiState, status: String) {
    val palette = LocalRelayPalette.current
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = androidx.compose.foundation.shape.RoundedCornerShape(18.dp),
        color = palette.surface,
        border = androidx.compose.foundation.BorderStroke(1.dp, palette.segment),
    ) {
        Column(Modifier.padding(16.dp), verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(12.dp)) {
            Text(
                stringResource(R.string.synthetic_panel_title),
                fontSize = 15.sp,
                fontWeight = FontWeight.SemiBold,
                color = palette.ink,
            )
            Text(
                stringResource(R.string.queue_counts, poc.counts.pending, poc.counts.synced, poc.counts.rejected),
                fontSize = 13.sp,
                color = palette.ink,
            )
            Text(
                stringResource(R.string.status_label, status),
                fontSize = 12.sp,
                color = palette.muted,
            )
            Text(
                stringResource(R.string.fixture_sms, "$SMS_PREFIX${poc.fixture}"),
                fontSize = 11.sp,
                color = palette.muted,
                lineHeight = 17.sp,
            )
            Row(horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(10.dp)) {
                PocButton(stringResource(R.string.persist_fixture)) { poc.persistFixture() }
                PocButton(stringResource(R.string.new_fixture)) { poc.newFixture() }
            }
            PocButton(stringResource(R.string.request_sync)) { poc.requestSync() }
            Text(
                stringResource(R.string.dedup_hint),
                fontSize = 11.sp,
                color = palette.muted,
            )
            SelectionContainer {
                Text(
                    stringResource(R.string.installation_id, poc.installationId ?: stringResource(R.string.loading)),
                    fontSize = 11.sp,
                    color = palette.ink,
                )
            }
            Text(
                stringResource(if (poc.permission) R.string.permission_granted else R.string.permission_not_granted),
                fontSize = 12.sp,
                color = palette.muted,
            )
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    stringResource(R.string.pause_uploads),
                    modifier = Modifier.weight(1f),
                    fontSize = 12.sp,
                    color = palette.ink,
                )
                Switch(checked = poc.paused, onCheckedChange = { poc.pauseUploads(it) })
            }
            Text(
                stringResource(R.string.synthetic_notice),
                fontSize = 11.sp,
                lineHeight = 16.sp,
                color = palette.muted,
            )
        }
    }
}

@Composable
private fun PocButton(label: String, onClick: () -> Unit) {
    val palette = LocalRelayPalette.current
    Button(
        onClick = onClick,
        colors = ButtonDefaults.buttonColors(containerColor = palette.ink, contentColor = palette.canvas),
        contentPadding = PaddingValues(horizontal = 14.dp, vertical = 6.dp),
    ) {
        Text(label, fontSize = 12.sp)
    }
}

@Composable
private fun SyntheticEventRow(event: StoredEvent) {
    val palette = LocalRelayPalette.current
    Column(Modifier.fillMaxWidth()) {
        Row(Modifier.fillMaxWidth().padding(vertical = 13.dp), verticalAlignment = Alignment.CenterVertically) {
            Surface(modifier = Modifier.size(38.dp), shape = CircleShape, color = palette.segment) {
                Box(contentAlignment = Alignment.Center) {
                    Text(
                        stringResource(
                            if (event.source == "sms_broadcast") {
                                R.string.synthetic_event_glyph_sms
                            } else {
                                R.string.synthetic_event_glyph_injected
                            },
                        ),
                        fontSize = 14.sp,
                        color = palette.ink,
                    )
                }
            }
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f), verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(3.dp)) {
                Text(
                    stringResource(if (event.source == "sms_broadcast") R.string.source_sms_broadcast else R.string.source_injected),
                    fontSize = 13.sp,
                    color = palette.ink,
                    fontWeight = FontWeight.Medium,
                )
                Text(
                    event.eventId,
                    fontSize = 10.sp,
                    color = palette.muted,
                    fontFamily = FontFamily.Monospace,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Text(DateFormat.format("MM-dd", event.receivedAtMs).toString(), fontSize = 10.sp, color = palette.muted)
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(palette.segment))
    }
}

/**
 * Sample cards, explicitly labelled demo data. They exist only inside this verification entry and
 * never pretend to be real conversations; they are not clickable into formal detail pages.
 */
@Composable
private fun DemoCardsSection() {
    val palette = LocalRelayPalette.current
    Column(Modifier.fillMaxWidth().padding(top = 24.dp)) {
        Text(
            stringResource(R.string.demo_section_title),
            fontSize = 13.sp,
            color = palette.muted,
        )
        Spacer(Modifier.height(6.dp))
        Text(
            stringResource(R.string.demo_section_notice),
            fontSize = 11.sp,
            lineHeight = 16.sp,
            color = palette.muted,
        )
        Spacer(Modifier.height(12.dp))
        SmsCard(
            data = SmsCardData(
                deviceName = stringResource(R.string.demo_sms_device),
                simLabel = stringResource(R.string.demo_sms_sim),
                confirmedLocalNumber = stringResource(R.string.demo_sms_number),
                sender = stringResource(R.string.demo_sms_sender),
                snippet = stringResource(R.string.demo_sms_snippet),
                timeText = stringResource(R.string.demo_sms_time),
                statusText = stringResource(R.string.demo_sms_status),
            ),
            isDemo = true,
        )
        Spacer(Modifier.height(10.dp))
        SmsCard(
            data = SmsCardData(
                deviceName = stringResource(R.string.demo_sms_device),
                simLabel = stringResource(R.string.demo_sms_sim),
                confirmedLocalNumber = null,
                sender = stringResource(R.string.demo_sms_sender_alt),
                snippet = stringResource(R.string.demo_sms_snippet_alt),
                timeText = stringResource(R.string.demo_sms_time_alt),
                statusText = stringResource(R.string.demo_sms_status),
            ),
            isDemo = true,
        )
        Spacer(Modifier.height(10.dp))
        MailCard(
            data = MailCardData(
                accountAddress = stringResource(R.string.demo_mail_account),
                sender = stringResource(R.string.demo_mail_sender),
                subject = stringResource(R.string.demo_mail_subject),
                snippet = stringResource(R.string.demo_mail_snippet),
                timeText = stringResource(R.string.demo_mail_time),
                hasAttachment = true,
                unread = true,
            ),
            isDemo = true,
        )
        Spacer(Modifier.height(28.dp))
    }
}
