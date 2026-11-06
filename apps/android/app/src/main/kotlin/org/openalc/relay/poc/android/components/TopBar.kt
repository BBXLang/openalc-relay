package org.openalc.relay.poc.android.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.ripple
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.navigation.RelayChannel
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/**
 * Formal top bar: left circular menu, central SMS/Mail segment with a sliding selected background,
 * right circular search button. The selected background position is driven by the shared channel
 * shift progress so tab and content slide together (plan section 2.2).
 */
@Composable
fun TopBar(
    shiftRatio: () -> Float,
    onMenu: () -> Unit,
    onChannelSelected: (RelayChannel) -> Unit,
    onSearch: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = LocalRelayPalette.current
    Row(
        modifier = modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CircleIconButton(onClick = onMenu, contentDescription = stringResource(R.string.menu_open)) {
            MenuGlyph(palette.ink)
        }
        BoxWithConstraints(
            modifier = Modifier
                .weight(1f)
                .padding(horizontal = 15.dp)
                .height(51.dp)
                .background(palette.segment, RoundedCornerShape(50))
                .padding(4.dp),
        ) {
            val tabWidth: Dp = maxWidth / 2
            Box(
                modifier = Modifier
                    .fillMaxHeight()
                    .width(tabWidth)
                    .offset(x = tabWidth * shiftRatio().coerceIn(0f, 1f))
                    .background(palette.surface, RoundedCornerShape(50)),
            )
            Row(Modifier.fillMaxSize()) {
                SegmentTab(
                    modifier = Modifier.weight(1f),
                    title = stringResource(R.string.channel_sms),
                    isSelected = shiftRatio() < 0.5f,
                    onClick = { onChannelSelected(RelayChannel.SMS) },
                )
                SegmentTab(
                    modifier = Modifier.weight(1f),
                    title = stringResource(R.string.channel_mail),
                    isSelected = shiftRatio() >= 0.5f,
                    onClick = { onChannelSelected(RelayChannel.MAIL) },
                )
            }
        }
        CircleIconButton(onClick = onSearch, contentDescription = stringResource(R.string.search)) {
            SearchGlyph(palette.ink)
        }
    }
}

@Composable
private fun SegmentTab(
    modifier: Modifier,
    title: String,
    isSelected: Boolean,
    onClick: () -> Unit,
) {
    val palette = LocalRelayPalette.current
    val interaction = remember { MutableInteractionSource() }
    Box(
        modifier = modifier
            .fillMaxHeight()
            // Clip before clickable so the ripple stays inside the pill instead of a square flash.
            .clip(RoundedCornerShape(50))
            .semantics {
                role = Role.Tab
                selected = isSelected
                contentDescription = title
            }
            .clickable(
                interactionSource = interaction,
                indication = ripple(bounded = true),
                onClick = onClick,
            ),
        contentAlignment = Alignment.Center,
    ) {
        Text(title, fontSize = 15.sp, color = palette.ink)
    }
}

/** Circular surface button used for menu/search and detail back arrows. */
@Composable
fun CircleIconButton(
    onClick: () -> Unit,
    contentDescription: String,
    modifier: Modifier = Modifier,
    containerColor: Color = LocalRelayPalette.current.surface,
    content: @Composable () -> Unit,
) {
    val interaction = remember { MutableInteractionSource() }
    Surface(
        modifier = modifier
            .size(48.dp)
            // Clip before clickable so the ripple stays inside the circle instead of a square flash.
            .clip(CircleShape)
            .semantics { this.contentDescription = contentDescription }
            .clickable(
                interactionSource = interaction,
                indication = ripple(bounded = true),
                onClick = onClick,
            ),
        shape = CircleShape,
        color = containerColor,
    ) {
        Box(contentAlignment = Alignment.Center) { content() }
    }
}

@Composable
fun MenuGlyph(color: Color) {
    Canvas(Modifier.fillMaxSize()) {
        val strokeWidth = 2.6.dp.toPx()
        drawLine(
            color,
            Offset(size.width * .30f, size.height * .42f),
            Offset(size.width * .70f, size.height * .42f),
            strokeWidth,
            StrokeCap.Round,
        )
        drawLine(
            color,
            Offset(size.width * .30f, size.height * .60f),
            Offset(size.width * .58f, size.height * .60f),
            strokeWidth,
            StrokeCap.Round,
        )
    }
}

@Composable
fun SearchGlyph(color: Color) {
    Canvas(Modifier.fillMaxSize()) {
        val strokeWidth = 2.4.dp.toPx()
        drawCircle(
            color = color,
            radius = size.minDimension * 0.24f,
            center = Offset(size.width * 0.42f, size.height * 0.42f),
            style = Stroke(strokeWidth),
        )
        drawLine(
            color = color,
            start = Offset(size.width * 0.60f, size.height * 0.60f),
            end = Offset(size.width * 0.82f, size.height * 0.82f),
            strokeWidth = strokeWidth,
            cap = StrokeCap.Round,
        )
    }
}

@Composable
fun BackChevronGlyph(color: Color) {
    Canvas(Modifier.fillMaxSize()) {
        val strokeWidth = 2.4.dp.toPx()
        drawPath(
            color = color,
            path = androidx.compose.ui.graphics.Path().apply {
                moveTo(size.width * 0.58f, size.height * 0.28f)
                lineTo(size.width * 0.36f, size.height * 0.5f)
                lineTo(size.width * 0.58f, size.height * 0.72f)
            },
            style = Stroke(width = strokeWidth, cap = StrokeCap.Round, join = androidx.compose.ui.graphics.StrokeJoin.Round),
        )
    }
}
