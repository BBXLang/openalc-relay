package org.openalc.relay.poc.android.screens

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.components.DetailTopBar
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/**
 * Mail account management page. The formal mail service is not connected, so no account can be
 * added or tested yet; the page explains what will be supported instead of faking accounts.
 */
@Composable
fun MailAccountsScreen(onBack: () -> Unit, modifier: Modifier = Modifier) {
    val palette = LocalRelayPalette.current
    Column(
        modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        DetailTopBar(title = stringResource(R.string.mail_accounts_title), onBack = onBack)
        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 28.dp, vertical = 12.dp),
        ) {
            Text(
                stringResource(R.string.mail_accounts_status),
                fontSize = 14.sp,
                fontWeight = FontWeight.SemiBold,
                color = palette.ink,
            )
            Spacer(Modifier.height(10.dp))
            Text(
                stringResource(R.string.mail_accounts_plan),
                fontSize = 12.sp,
                lineHeight = 18.sp,
                color = palette.muted,
            )
            Spacer(Modifier.height(18.dp))
            Button(
                onClick = {},
                enabled = false,
                colors = ButtonDefaults.buttonColors(
                    containerColor = palette.ink,
                    contentColor = palette.canvas,
                    disabledContainerColor = palette.segment,
                    disabledContentColor = palette.muted,
                ),
            ) {
                Text(stringResource(R.string.mail_accounts_add), fontSize = 13.sp)
            }
            Spacer(Modifier.height(8.dp))
            Text(
                stringResource(R.string.mail_accounts_add_disabled),
                fontSize = 11.sp,
                color = palette.muted,
            )
        }
    }
}
