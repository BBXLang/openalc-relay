package org.openalc.relay.poc.android.navigation

import android.provider.Settings
import androidx.activity.compose.BackHandler
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDrawerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.components.ComposeDock
import org.openalc.relay.poc.android.components.RelayDrawerContent
import org.openalc.relay.poc.android.components.TopBar
import org.openalc.relay.poc.android.poc.PocUiState
import org.openalc.relay.poc.android.screens.DeviceDetailScreen
import org.openalc.relay.poc.android.screens.MailAccountsScreen
import org.openalc.relay.poc.android.screens.MailDetailScreen
import org.openalc.relay.poc.android.screens.SearchScreen
import org.openalc.relay.poc.android.screens.SettingsScreen
import org.openalc.relay.poc.android.screens.SmsDetailScreen
import org.openalc.relay.poc.android.screens.SmsListScreen
import org.openalc.relay.poc.android.screens.SyntheticVerificationScreen
import org.openalc.relay.poc.android.screens.MailListScreen
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/**
 * Navigation host with a self-managed back stack (no navigation library). System back pops one
 * destination at a time; the home screen keeps both channel lists composed so returning from a
 * detail, search or settings page restores the original list and scroll state.
 */
@Composable
fun RelayApp(state: RelayAppState, poc: PocUiState) {
    val drawerState = rememberDrawerState(DrawerValue.Closed)
    val scope = rememberCoroutineScope()
    val openDrawer: () -> Unit = { scope.launch { drawerState.open() } }
    // Navigating from the drawer always closes it first (device-verified inconsistency, 2026-11-06).
    val openFromDrawer: (RelayDestination) -> Unit = { destination ->
        scope.launch { drawerState.close() }
        state.open(destination)
    }

    BackHandler(enabled = state.canGoBack) { state.back() }

    ModalNavigationDrawer(
        drawerState = drawerState,
        drawerContent = {
            RelayDrawerContent(
                devices = emptyList(),
                accounts = emptyList(),
                onDeviceClick = { id -> openFromDrawer(RelayDestination.DeviceDetail(id)) },
                onSelectAccount = {
                    scope.launch { drawerState.close() }
                    state.switchChannel(RelayChannel.MAIL)
                },
                onAddAccount = { openFromDrawer(RelayDestination.MailAccounts) },
                onOpenSettings = { openFromDrawer(RelayDestination.Settings) },
            )
        },
    ) {
        // Paint the canvas so transparent list areas never expose the (light) window background
        // in dark mode (device-verified defect D1, 2026-11-06).
        Box(
            Modifier
                .fillMaxSize()
                .background(LocalRelayPalette.current.canvas),
        ) {
            when (val top = state.top()) {
                is RelayDestination.SmsList, is RelayDestination.MailList ->
                    HomeScreen(state = state, onMenu = openDrawer)
                is RelayDestination.Search -> SearchScreen(channel = top.channel, onBack = { state.back() })
                is RelayDestination.SmsDetail -> SmsDetailScreen(id = top.id, onBack = { state.back() })
                is RelayDestination.MailDetail -> MailDetailScreen(id = top.id, onBack = { state.back() })
                is RelayDestination.DeviceDetail -> DeviceDetailScreen(id = top.id, onBack = { state.back() })
                is RelayDestination.MailAccounts -> MailAccountsScreen(onBack = { state.back() })
                is RelayDestination.Settings -> SettingsScreen(
                    state = state,
                    onBack = { state.back() },
                    onOpenMailAccounts = { state.open(RelayDestination.MailAccounts) },
                    onOpenSyntheticVerification = { state.open(RelayDestination.SyntheticVerification) },
                )
                is RelayDestination.SyntheticVerification ->
                    SyntheticVerificationScreen(poc = poc, recentEvents = poc.recentEvents, onBack = { state.back() })
            }
        }
    }
}

/**
 * Home screen: top bar with SMS/Mail segment, swipeable list content area and the compose dock.
 * Clicking a tab or horizontally swiping the list content area switches channel with a ~250 ms
 * slide (0 ms when the system has animations removed); inputs, the drawer and detail pages are
 * outside the gesture area and never trigger the switch.
 */
@Composable
fun HomeScreen(state: RelayAppState, onMenu: () -> Unit) {
    val animationMillis = rememberChannelAnimationMillis()
    val scope = rememberCoroutineScope()
    val shift = remember { ChannelShiftState(state.channel, scope) }
    var showSourceNotice by remember { mutableStateOf(false) }

    // Drive the shared slide progress whenever the channel changes (tab click or swipe commit).
    LaunchedEffect(state.channel, animationMillis) {
        shift.animateTo(state.channel, animationMillis)
    }

    Column(
        Modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        TopBar(
            shiftRatio = { shift.ratio() },
            onMenu = onMenu,
            onChannelSelected = { target -> state.switchChannel(target) },
            onSearch = { state.open(RelayDestination.Search(state.channel)) },
        )
        ChannelPager(state = state, shift = shift, modifier = Modifier.weight(1f))
        ComposeDock(
            recipientHint = channelText(state.channel, R.string.compose_recipient_sms, R.string.compose_recipient_mail),
            recipient = if (state.channel == RelayChannel.SMS) state.smsRecipient else state.mailRecipient,
            onRecipientChange = if (state.channel == RelayChannel.SMS) state::updateSmsRecipient else state::updateMailRecipient,
            bodyHint = channelText(state.channel, R.string.compose_body_sms, R.string.compose_body_mail),
            body = if (state.channel == RelayChannel.SMS) state.smsBody else state.mailBody,
            onBodyChange = if (state.channel == RelayChannel.SMS) state::updateSmsBody else state::updateMailBody,
            sourceSummary = if (state.channel == RelayChannel.SMS) state.smsSourceSummary else state.mailSourceSummary,
            isSmsChannel = state.channel == RelayChannel.SMS,
            onPickSource = { showSourceNotice = true },
        )
    }

    if (showSourceNotice) {
        AlertDialog(
            onDismissRequest = { showSourceNotice = false },
            title = {
                Text(
                    stringResource(
                        if (state.channel == RelayChannel.SMS) {
                            R.string.compose_pick_source_sms
                        } else {
                            R.string.compose_pick_source_mail
                        },
                    ),
                    fontSize = 17.sp,
                )
            },
            text = { Text(stringResource(R.string.compose_source_unavailable), fontSize = 13.sp) },
            confirmButton = {
                TextButton(onClick = { showSourceNotice = false }) {
                    Text(stringResource(R.string.dialog_ok))
                }
            },
        )
    }
}

@Composable
private fun channelText(channel: RelayChannel, smsRes: Int, mailRes: Int): String =
    stringResource(if (channel == RelayChannel.SMS) smsRes else mailRes)

/**
 * Sliding content area for both channel lists. SMS page slides out left while the mail page slides
 * in from the right and vice versa, using the shared shift progress.
 */
@Composable
private fun ChannelPager(state: RelayAppState, shift: ChannelShiftState, modifier: Modifier = Modifier) {
    Box(
        modifier
            .fillMaxWidth()
            .onSizeChanged { shift.onContentWidthChanged(it.width.toFloat()) }
            .pointerInput(Unit) {
                detectHorizontalDragGestures(
                    onHorizontalDrag = { change, dragAmount ->
                        change.consume()
                        shift.onDrag(dragAmount)
                    },
                    onDragEnd = {
                        val commit = shift.commitDrag(state.channel)
                        if (commit != null) {
                            state.switchChannel(commit)
                        } else {
                            shift.cancel(state.channel)
                        }
                    },
                    onDragCancel = { shift.cancel(state.channel) },
                )
            },
    ) {
        Box(
            Modifier
                .fillMaxSize()
                .graphicsLayer { translationX = -shift.ratio() * size.width },
        ) {
            SmsListScreen(listState = state.smsListState)
        }
        Box(
            Modifier
                .fillMaxSize()
                .graphicsLayer { translationX = (1f - shift.ratio()) * size.width },
        ) {
            MailListScreen(listState = state.mailListState)
        }
    }
}

/**
 * Shared slide progress for the top bar segment background and the list content area.
 * `visualRatio` is the single source of truth: 0 for SMS, 1 for mail, and every intermediate value
 * while dragging. Animatable is always snapped to the exact visual position before animating, so a
 * committed swipe continues smoothly from where the finger left off instead of jumping back first.
 */
class ChannelShiftState(initialChannel: RelayChannel, private val scope: CoroutineScope) {
    private val progress = Animatable(if (initialChannel == RelayChannel.MAIL) 1f else 0f)

    private var visualRatio by mutableFloatStateOf(
        if (initialChannel == RelayChannel.MAIL) 1f else 0f,
    )
    private var contentWidthPx: Float = 1f

    /** Current visual ratio for rendering, clamped to [0, 1]. */
    fun ratio(): Float = visualRatio.coerceIn(0f, 1f)

    fun onContentWidthChanged(widthPx: Float) {
        contentWidthPx = widthPx.coerceAtLeast(1f)
    }

    /** dragAmountPx is the finger movement toward mail (negative) or back toward SMS (positive). */
    fun onDrag(dragAmountPx: Float) {
        visualRatio = (visualRatio - dragAmountPx / contentWidthPx).coerceIn(0f, 1f)
    }

    /** Ends the drag; returns the channel to commit, or null when the swipe is below the threshold. */
    fun commitDrag(current: RelayChannel): RelayChannel? = when (current) {
        RelayChannel.SMS -> if (ratio() > COMMIT_THRESHOLD) RelayChannel.MAIL else null
        RelayChannel.MAIL -> if (ratio() < 1f - COMMIT_THRESHOLD) RelayChannel.SMS else null
    }

    /** Below-threshold end or cancellation: keep the visual position and slide back to `channel`. */
    fun cancel(channel: RelayChannel) {
        scope.launch { animateTo(channel, CHANNEL_ANIMATION_MILLIS) }
    }

    suspend fun animateTo(channel: RelayChannel, durationMillis: Int) {
        val target = if (channel == RelayChannel.MAIL) 1f else 0f
        // Start from the exact visual position so a committed drag never jumps back first.
        progress.snapTo(ratio())
        if (durationMillis <= 0) {
            progress.snapTo(target)
        } else {
            progress.animateTo(target, tween(durationMillis))
        }
        visualRatio = target
    }

    private companion object {
        const val COMMIT_THRESHOLD = 0.2f
    }
}

/** ~250 ms channel slide; 0 when the system has "remove animations" enabled. */
@Composable
fun rememberChannelAnimationMillis(): Int {
    val context = LocalContext.current
    return remember {
        val scale = Settings.Global.getFloat(
            context.contentResolver,
            Settings.Global.ANIMATOR_DURATION_SCALE,
            1f,
        )
        if (scale == 0f) 0 else CHANNEL_ANIMATION_MILLIS
    }
}

private const val CHANNEL_ANIMATION_MILLIS = 250
