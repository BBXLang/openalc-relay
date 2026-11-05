package org.openalc.relay.poc.android.components

import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/** Simple sub-page top bar: circular back button plus the page title. */
@Composable
fun DetailTopBar(title: String, onBack: () -> Unit, modifier: Modifier = Modifier) {
    val palette = LocalRelayPalette.current
    Row(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CircleIconButton(
            onClick = onBack,
            contentDescription = stringResource(R.string.back),
        ) {
            BackChevronGlyph(palette.ink)
        }
        Text(
            title,
            modifier = Modifier.padding(start = 14.dp),
            fontSize = 19.sp,
            fontWeight = FontWeight.SemiBold,
            color = palette.ink,
        )
    }
}
