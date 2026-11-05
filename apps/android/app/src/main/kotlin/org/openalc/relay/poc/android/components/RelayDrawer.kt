package org.openalc.relay.poc.android.components

import androidx.compose.animation.animateContentSize
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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/** Summary shown in the "devices" drawer group; empty until formal pairing is connected. */
data class DeviceSummary(
    val id: String,
    val name: String,
    val simLabels: List<String>,
    /** Recognized SIMs, permissions, last observation and revoke state, when known. */
    val statusText: String? = null,
)

/** Summary shown in the "mail" drawer group; empty until formal accounts exist. */
data class MailAccountSummary(
    val id: String,
    val address: String,
    val connectionStateText: String,
)

/**
 * Drawer content per plan section 2.2: text logo (Relay / by OpenALC), independently expandable
 * "devices" and "mail" groups, and a settings entry. PoC operations intentionally live only in the
 * synthetic verification screen.
 */
@Composable
fun RelayDrawerContent(
    devices: List<DeviceSummary>,
    accounts: List<MailAccountSummary>,
    onDeviceClick: (String) -> Unit,
    onSelectAccount: (String) -> Unit,
    onAddAccount: () -> Unit,
    onOpenSettings: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = LocalRelayPalette.current
    var devicesExpanded by remember { mutableStateOf(true) }
    var mailExpanded by remember { mutableStateOf(false) }
    var showPairingNotice by remember { mutableStateOf(false) }

    Column(
        modifier = modifier
            .width(320.dp)
            .fillMaxSize()
            .background(palette.canvas)
            .statusBarsPadding()
            .verticalScroll(rememberScrollState())
            .padding(22.dp),
    ) {
        // Text logo: "Relay" on its own line with ~0.14em letter spacing, "by OpenALC" bottom right
        // at about 45% of the size, secondary color.
        Column(Modifier.fillMaxWidth()) {
            Text(
                "Relay",
                fontSize = 23.sp,
                fontWeight = FontWeight.SemiBold,
                letterSpacing = 0.14.em,
                color = palette.ink,
            )
            Text(
                stringResource(R.string.drawer_by_openalc),
                modifier = Modifier.align(Alignment.End),
                fontSize = 10.sp,
                color = palette.muted,
            )
        }
        Spacer(Modifier.height(26.dp))
        DrawerGroup(
            title = stringResource(R.string.drawer_devices),
            expanded = devicesExpanded,
            onToggle = { devicesExpanded = !devicesExpanded },
        ) {
            if (devices.isEmpty()) {
                Text(
                    stringResource(R.string.drawer_no_devices),
                    fontSize = 12.sp,
                    color = palette.muted,
                    modifier = Modifier.padding(start = 10.dp, top = 4.dp),
                )
                DrawerActionRow(stringResource(R.string.drawer_pair_device)) { showPairingNotice = true }
            } else {
                devices.forEach { device ->
                    val simLine = if (device.simLabels.isEmpty()) {
                        null
                    } else {
                        stringResource(R.string.drawer_sim_labels, device.simLabels.joinToString("、"))
                    }
                    DrawerItemRow(
                        title = device.name,
                        subtitle = listOfNotNull(simLine, device.statusText).joinToString(" · ").ifEmpty { null },
                        onClick = { onDeviceClick(device.id) },
                    )
                }
            }
        }
        Spacer(Modifier.height(14.dp))
        DrawerGroup(
            title = stringResource(R.string.drawer_mail),
            expanded = mailExpanded,
            onToggle = { mailExpanded = !mailExpanded },
        ) {
            if (accounts.isEmpty()) {
                Text(
                    stringResource(R.string.drawer_no_accounts),
                    fontSize = 12.sp,
                    color = palette.muted,
                    modifier = Modifier.padding(start = 10.dp, top = 4.dp),
                )
                DrawerActionRow(stringResource(R.string.drawer_add_account)) { onAddAccount() }
            } else {
                accounts.forEach { account ->
                    DrawerItemRow(
                        title = account.address,
                        subtitle = stringResource(R.string.drawer_account_status, account.connectionStateText),
                        onClick = { onSelectAccount(account.id) },
                    )
                }
            }
        }
        Spacer(Modifier.height(14.dp))
        DrawerActionRow(stringResource(R.string.drawer_settings)) { onOpenSettings() }
        Spacer(Modifier.height(22.dp))
        Text(
            stringResource(R.string.drawer_footer_notice),
            fontSize = 11.sp,
            lineHeight = 16.sp,
            color = palette.muted,
        )
    }

    if (showPairingNotice) {
        AlertDialog(
            onDismissRequest = { showPairingNotice = false },
            title = { Text(stringResource(R.string.drawer_pair_device), fontSize = 17.sp) },
            text = { Text(stringResource(R.string.drawer_pairing_unavailable), fontSize = 13.sp) },
            confirmButton = {
                TextButton(onClick = { showPairingNotice = false }) {
                    Text(stringResource(R.string.dialog_ok))
                }
            },
        )
    }
}

@Composable
private fun DrawerGroup(
    title: String,
    expanded: Boolean,
    onToggle: () -> Unit,
    content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit,
) {
    val palette = LocalRelayPalette.current
    Column(
        Modifier
            .fillMaxWidth()
            .animateContentSize(),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .clickable(onClick = onToggle)
                .padding(vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                title,
                modifier = Modifier.weight(1f),
                fontSize = 15.sp,
                fontWeight = FontWeight.Medium,
                color = palette.ink,
            )
            ChevronGlyph(expanded, palette.ink)
        }
        if (expanded) {
            Column(Modifier.padding(start = 10.dp), content = content)
        }
    }
}

@Composable
private fun DrawerItemRow(title: String, subtitle: String?, onClick: () -> Unit) {
    val palette = LocalRelayPalette.current
    Column(
        Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(vertical = 8.dp),
    ) {
        Text(
            title,
            fontSize = 13.sp,
            color = palette.ink,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (subtitle != null) {
            Text(
                subtitle,
                fontSize = 11.sp,
                color = palette.muted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

/** Row offering an action (pair a device, add an account, open settings). */
@Composable
private fun DrawerActionRow(label: String, onClick: () -> Unit) {
    val palette = LocalRelayPalette.current
    Box(
        modifier = Modifier
            .padding(top = 6.dp, bottom = 4.dp)
            .background(palette.segment, RoundedCornerShape(50))
            .clickable(onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 8.dp),
    ) {
        Text(label, fontSize = 12.sp, color = palette.ink)
    }
}

@Composable
private fun ChevronGlyph(expanded: Boolean, color: androidx.compose.ui.graphics.Color) {
    Canvas(Modifier.size(18.dp)) {
        val stroke = Stroke(width = 2.dp.toPx(), cap = StrokeCap.Round)
        val path = androidx.compose.ui.graphics.Path()
        if (expanded) {
            path.moveTo(size.width * 0.25f, size.height * 0.4f)
            path.lineTo(size.width * 0.5f, size.height * 0.65f)
            path.lineTo(size.width * 0.75f, size.height * 0.4f)
        } else {
            path.moveTo(size.width * 0.4f, size.height * 0.25f)
            path.lineTo(size.width * 0.65f, size.height * 0.5f)
            path.lineTo(size.width * 0.4f, size.height * 0.75f)
        }
        drawPath(path, color, style = stroke)
    }
}
