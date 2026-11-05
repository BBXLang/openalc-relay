package org.openalc.relay.poc.android.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.RadioButton
import androidx.compose.material3.RadioButtonDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.components.DetailTopBar
import org.openalc.relay.poc.android.navigation.RelayAppState
import org.openalc.relay.poc.android.theme.LocalRelayPalette
import org.openalc.relay.poc.android.theme.ThemeMode

/**
 * Global settings page (plan section 2.1). Theme override is real and persisted; pairing/revocation
 * and security/sync entries are honest placeholders that explain the formal link is not connected —
 * no fake switches. The synthetic verification entry is the only way into the PoC screens.
 */
@Composable
fun SettingsScreen(
    state: RelayAppState,
    onBack: () -> Unit,
    onOpenMailAccounts: () -> Unit,
    onOpenSyntheticVerification: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = LocalRelayPalette.current
    Column(
        modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        DetailTopBar(title = stringResource(R.string.settings_title), onBack = onBack)
        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 22.dp, vertical = 8.dp),
        ) {
            SettingsSectionLabel(stringResource(R.string.settings_appearance))
            SettingsCard {
                ThemeOptionRow(
                    label = stringResource(R.string.theme_light),
                    selected = state.themeMode == ThemeMode.LIGHT,
                    onSelect = { state.updateThemeMode(ThemeMode.LIGHT) },
                )
                ThemeOptionRow(
                    label = stringResource(R.string.theme_dark),
                    selected = state.themeMode == ThemeMode.DARK,
                    onSelect = { state.updateThemeMode(ThemeMode.DARK) },
                )
                ThemeOptionRow(
                    label = stringResource(R.string.theme_system),
                    selected = state.themeMode == ThemeMode.SYSTEM,
                    onSelect = { state.updateThemeMode(ThemeMode.SYSTEM) },
                )
            }
            Spacer(Modifier.height(18.dp))
            SettingsSectionLabel(stringResource(R.string.settings_services))
            SettingsCard {
                SettingsRow(
                    title = stringResource(R.string.settings_pairing),
                    trailingText = stringResource(R.string.not_connected_short),
                )
                SettingsRow(
                    title = stringResource(R.string.settings_mail_accounts),
                    trailingText = null,
                    onClick = onOpenMailAccounts,
                )
                SettingsRow(
                    title = stringResource(R.string.settings_security_sync),
                    trailingText = stringResource(R.string.settings_security_unavailable),
                )
            }
            Spacer(Modifier.height(18.dp))
            SettingsSectionLabel(stringResource(R.string.settings_developer))
            SettingsCard {
                SettingsRow(
                    title = stringResource(R.string.settings_synthetic_verification),
                    trailingText = null,
                    onClick = onOpenSyntheticVerification,
                )
            }
            Spacer(Modifier.height(24.dp))
            Text(
                stringResource(R.string.settings_footer_notice),
                fontSize = 11.sp,
                lineHeight = 16.sp,
                color = palette.muted,
            )
            Spacer(Modifier.height(24.dp))
        }
    }
}

@Composable
private fun SettingsSectionLabel(label: String) {
    Text(
        label,
        modifier = Modifier.padding(start = 6.dp, bottom = 8.dp),
        fontSize = 12.sp,
        color = LocalRelayPalette.current.muted,
    )
}

@Composable
private fun SettingsCard(content: @Composable () -> Unit) {
    Column(
        Modifier
            .fillMaxWidth()
            .background(LocalRelayPalette.current.surface, RoundedCornerShape(18.dp))
            .padding(vertical = 4.dp),
        content = { content() },
    )
}

@Composable
private fun ThemeOptionRow(label: String, selected: Boolean, onSelect: () -> Unit) {
    val palette = LocalRelayPalette.current
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onSelect)
            .padding(horizontal = 14.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RadioButton(
            selected = selected,
            onClick = onSelect,
            colors = RadioButtonDefaults.colors(selectedColor = palette.ink, unselectedColor = palette.muted),
        )
        Text(label, fontSize = 14.sp, color = palette.ink)
    }
}

@Composable
private fun SettingsRow(
    title: String,
    trailingText: String?,
    onClick: (() -> Unit)? = null,
) {
    val palette = LocalRelayPalette.current
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .padding(horizontal = 14.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            title,
            modifier = Modifier.weight(1f),
            fontSize = 14.sp,
            color = palette.ink,
        )
        if (trailingText != null) {
            Text(trailingText, fontSize = 12.sp, color = palette.muted)
        }
    }
}
