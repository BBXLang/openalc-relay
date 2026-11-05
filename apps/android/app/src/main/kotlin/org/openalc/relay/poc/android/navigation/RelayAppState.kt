package org.openalc.relay.poc.android.navigation

import android.content.Context
import android.content.SharedPreferences
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.foundation.lazy.LazyListState
import org.openalc.relay.poc.android.theme.ThemeMode

/**
 * Self-managed navigation state (no navigation library dependency).
 *
 * - Back stack of [RelayDestination]; system back pops one level and list screens keep their
 *   scroll/detail state because [smsListState] / [mailListState] live here instead of in the screens.
 * - Drafts are stored per channel in SharedPreferences with the channel in the key so switching
 *   channels never overwrites the other channel's draft.
 *   NOTE: 真实正文草稿在正式链路开放时须加密保存；当前仅保存用户手动输入的未接通草稿。
 */
class RelayAppState(context: Context) {
    private val preferences: SharedPreferences = context.getSharedPreferences("relay-ui", Context.MODE_PRIVATE)

    val stack = mutableStateListOf<RelayDestination>(RelayDestination.SmsList)

    /** Scroll state per channel so back from details restores the original list position. */
    val smsListState = LazyListState()
    val mailListState = LazyListState()

    var themeMode by mutableStateOf(loadThemeMode())
        private set

    var channel by mutableStateOf(RelayChannel.SMS)
        private set

    // Drafts, keys contain the channel name on purpose.
    var smsRecipient by mutableStateOf(preferences.getString(KEY_SMS_RECIPIENT, "").orEmpty())
    var smsBody by mutableStateOf(preferences.getString(KEY_SMS_BODY, "").orEmpty())
    var mailRecipient by mutableStateOf(preferences.getString(KEY_MAIL_RECIPIENT, "").orEmpty())
    var mailBody by mutableStateOf(preferences.getString(KEY_MAIL_BODY, "").orEmpty())

    /** Current device/SIM or sender account summary shown in the dock; null until formally connected. */
    var smsSourceSummary by mutableStateOf<String?>(null)
    var mailSourceSummary by mutableStateOf<String?>(null)

    val canGoBack: Boolean get() = stack.size > 1

    fun top(): RelayDestination = stack.last()

    fun open(destination: RelayDestination) {
        stack.add(destination)
    }

    /** Pops one level; returns false when already at the root so the system can finish the activity. */
    fun back(): Boolean {
        if (stack.size <= 1) return false
        stack.removeAt(stack.lastIndex)
        return true
    }

    /**
     * Tab switch: keeps one home entry on the stack and mirrors the visible channel into it,
     * so the top of the stack always matches what the user sees.
     */
    fun switchChannel(target: RelayChannel) {
        if (channel == target) return
        channel = target
        val homeDestination = if (target == RelayChannel.SMS) RelayDestination.SmsList else RelayDestination.MailList
        val currentTop = stack.last()
        if (currentTop is RelayDestination.SmsList || currentTop is RelayDestination.MailList) {
            stack[stack.lastIndex] = homeDestination
        }
    }

    fun updateThemeMode(mode: ThemeMode) {
        themeMode = mode
        preferences.edit().putString(KEY_THEME_MODE, mode.name).apply()
    }

    fun updateSmsRecipient(value: String) {
        smsRecipient = value
        preferences.edit().putString(KEY_SMS_RECIPIENT, value).apply()
    }

    fun updateSmsBody(value: String) {
        smsBody = value
        preferences.edit().putString(KEY_SMS_BODY, value).apply()
    }

    fun updateMailRecipient(value: String) {
        mailRecipient = value
        preferences.edit().putString(KEY_MAIL_RECIPIENT, value).apply()
    }

    fun updateMailBody(value: String) {
        mailBody = value
        preferences.edit().putString(KEY_MAIL_BODY, value).apply()
    }

    private fun loadThemeMode(): ThemeMode {
        val stored = preferences.getString(KEY_THEME_MODE, null) ?: return ThemeMode.SYSTEM
        return runCatching { ThemeMode.valueOf(stored) }.getOrDefault(ThemeMode.SYSTEM)
    }

    private companion object {
        const val KEY_THEME_MODE = "theme-mode"
        const val KEY_SMS_RECIPIENT = "draft-recipient-sms"
        const val KEY_SMS_BODY = "draft-body-sms"
        const val KEY_MAIL_RECIPIENT = "draft-recipient-mail"
        const val KEY_MAIL_BODY = "draft-body-mail"
    }
}
