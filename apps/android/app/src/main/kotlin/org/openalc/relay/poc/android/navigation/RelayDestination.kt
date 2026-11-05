package org.openalc.relay.poc.android.navigation

/**
 * Destinations from the formal UI plan section 2.1 navigation table.
 * No navigation library is added; the host keeps its own back stack (see [RelayAppState]).
 */
sealed class RelayDestination {
    data object SmsList : RelayDestination()
    data object MailList : RelayDestination()
    data class Search(val channel: RelayChannel) : RelayDestination()
    data class SmsDetail(val id: String) : RelayDestination()
    data class MailDetail(val id: String) : RelayDestination()
    data class DeviceDetail(val id: String) : RelayDestination()
    data object MailAccounts : RelayDestination()
    data object Settings : RelayDestination()
    data object SyntheticVerification : RelayDestination()
}

enum class RelayChannel { SMS, MAIL }
