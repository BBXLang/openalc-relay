package org.openalc.relay.poc.android.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

/**
 * Formal UI tokens from docs/development/FORMAL-UI-MAIL-PLAN.zh-CN.md section 2.2.
 * Light: canvas #FAFAFA, surface #FFFFFF, ink #111111, muted #838383, segment #E9E9E9, accent #FFD12E.
 * Dark: bg #111214, surface #1D1E20, ink #F4F4F5, muted #A1A1AA, segment #2B2C2F.
 */
@Immutable
data class RelayPalette(
    val isDark: Boolean,
    val canvas: Color,
    val surface: Color,
    val ink: Color,
    val muted: Color,
    val segment: Color,
    val yellow: Color,
    val contentOnYellow: Color,
)

val lightPalette = RelayPalette(
    isDark = false,
    canvas = Color(0xFFFAFAFA),
    surface = Color(0xFFFFFFFF),
    ink = Color(0xFF111111),
    muted = Color(0xFF838383),
    segment = Color(0xFFE9E9E9),
    yellow = Color(0xFFFFD12E),
    contentOnYellow = Color(0xFF111111),
)

val darkPalette = RelayPalette(
    isDark = true,
    canvas = Color(0xFF111214),
    surface = Color(0xFF1D1E20),
    ink = Color(0xFFF4F4F5),
    muted = Color(0xFFA1A1AA),
    segment = Color(0xFF2B2C2F),
    yellow = Color(0xFFFFD12E),
    contentOnYellow = Color(0xFF111214),
)

val LocalRelayPalette = staticCompositionLocalOf { lightPalette }

/** Theme override persisted by the settings screen; SYSTEM follows the OS setting. */
enum class ThemeMode { LIGHT, DARK, SYSTEM }

@Composable
fun RelayTheme(themeMode: ThemeMode, content: @Composable () -> Unit) {
    val isDark = when (themeMode) {
        ThemeMode.LIGHT -> false
        ThemeMode.DARK -> true
        ThemeMode.SYSTEM -> isSystemInDarkTheme()
    }
    val palette = if (isDark) darkPalette else lightPalette
    val scheme = if (isDark) {
        darkColorScheme(
            primary = palette.yellow,
            onPrimary = palette.contentOnYellow,
            secondary = palette.muted,
            onSecondary = palette.canvas,
            background = palette.canvas,
            onBackground = palette.ink,
            surface = palette.surface,
            onSurface = palette.ink,
            surfaceVariant = palette.segment,
            onSurfaceVariant = palette.muted,
            outline = palette.muted,
        )
    } else {
        lightColorScheme(
            primary = palette.yellow,
            onPrimary = palette.contentOnYellow,
            secondary = palette.muted,
            onSecondary = palette.canvas,
            background = palette.canvas,
            onBackground = palette.ink,
            surface = palette.surface,
            onSurface = palette.ink,
            surfaceVariant = palette.segment,
            onSurfaceVariant = palette.muted,
            outline = palette.muted,
        )
    }
    CompositionLocalProvider(LocalRelayPalette provides palette) {
        MaterialTheme(colorScheme = scheme, content = content)
    }
}
