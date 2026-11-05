package org.openalc.relay.poc.android

import android.Manifest
import android.content.pm.PackageManager
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.remember
import org.openalc.relay.poc.android.navigation.RelayApp
import org.openalc.relay.poc.android.navigation.RelayAppState
import org.openalc.relay.poc.android.poc.PocUiState
import org.openalc.relay.poc.android.theme.RelayTheme
import org.openalc.relay.poc.android.theme.ThemeMode

/**
 * Hosts the formal Relay UI (self-managed navigation, no navigation library) and the theme.
 * All former PoC operations live behind the synthetic verification destination; the PoC
 * application logic (Room queue, WorkManager workers, SyntheticSmsReceiver) is untouched.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val application = application as PocApplication
        val permissionGranted =
            checkSelfPermission(Manifest.permission.RECEIVE_SMS) == PackageManager.PERMISSION_GRANTED
        setContent {
            val state = remember { RelayAppState(applicationContext) }
            val poc = remember { PocUiState(application, permissionGranted) }
            LaunchedEffect(poc) { poc.observeIdentityAndCounts() }
            // Keep the transparent system bars readable in both themes and honour the override.
            val useDarkIcons = when (state.themeMode) {
                ThemeMode.LIGHT -> true
                ThemeMode.DARK -> false
                ThemeMode.SYSTEM -> !isSystemInDarkTheme()
            }
            SideEffect {
                val style = if (useDarkIcons) {
                    SystemBarStyle.light(
                        android.graphics.Color.TRANSPARENT,
                        android.graphics.Color.TRANSPARENT,
                    )
                } else {
                    SystemBarStyle.dark(android.graphics.Color.TRANSPARENT)
                }
                enableEdgeToEdge(statusBarStyle = style, navigationBarStyle = style)
            }
            RelayTheme(themeMode = state.themeMode) {
                RelayApp(state = state, poc = poc)
            }
        }
    }
}
