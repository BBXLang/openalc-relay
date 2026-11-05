package org.openalc.relay.poc.android

import android.text.format.DateFormat
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalDrawerSheet
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.lightColorScheme
import androidx.compose.material3.rememberDrawerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import org.openalc.relay.poc.QueueCounts
import org.openalc.relay.poc.SMS_PREFIX
import org.openalc.relay.poc.StoredEvent

private val canvas = Color(0xFFFAFAFA)
private val ink = Color(0xFF111111)
private val muted = Color(0xFF838383)
private val segment = Color(0xFFE9E9E9)
private val yellow = Color(0xFFFFD12E)

@Composable
fun RelayInboxScreen(
    counts: QueueCounts,
    recent: List<StoredEvent>,
    installationId: String?,
    status: String,
    permission: Boolean,
    paused: Boolean,
    fixture: String,
    onPauseChanged: (Boolean) -> Unit,
    onNewFixture: () -> Unit,
    onPersistFixture: () -> Unit,
    onRequestSync: () -> Unit,
) {
    var tab by remember { mutableStateOf(0) }
    var query by remember { mutableStateOf("") }
    val drawer = rememberDrawerState(DrawerValue.Closed)
    val scope = rememberCoroutineScope()
    val visible = remember(recent, query) {
        if (query.isBlank()) recent else recent.filter { it.eventId.contains(query.trim(), ignoreCase = true) }
    }

    MaterialTheme(colorScheme = lightColorScheme(background = canvas, surface = Color.White, onSurface = ink)) {
        ModalNavigationDrawer(
            drawerState = drawer,
            drawerContent = {
                ModalDrawerSheet(modifier = Modifier.width(320.dp), drawerContainerColor = canvas) {
                    LazyColumn(
                        modifier = Modifier.fillMaxWidth(),
                        contentPadding = PaddingValues(22.dp),
                        verticalArrangement = Arrangement.spacedBy(17.dp),
                    ) {
                        item { Text("Relay", fontSize = 23.sp, fontWeight = FontWeight.SemiBold, color = ink) }
                        item { Text("本机合成验证", fontSize = 12.sp, color = muted) }
                        item { Text(stringResource(R.string.queue_counts, counts.pending, counts.synced, counts.rejected), fontSize = 13.sp, color = ink) }
                        item { Text(stringResource(R.string.status_label, status), fontSize = 12.sp, color = muted) }
                        item { Text(stringResource(R.string.fixture_sms, "$SMS_PREFIX$fixture"), fontSize = 11.sp, color = muted, lineHeight = 17.sp) }
                        item { DrawerAction(stringResource(R.string.persist_fixture), onPersistFixture) }
                        item { DrawerAction(stringResource(R.string.new_fixture), onNewFixture) }
                        item { DrawerAction(stringResource(R.string.request_sync), onRequestSync) }
                        item { Text(stringResource(R.string.dedup_hint), fontSize = 11.sp, color = muted) }
                        item {
                            SelectionContainer {
                                Text(stringResource(R.string.installation_id, installationId ?: stringResource(R.string.loading)), fontSize = 11.sp, color = ink)
                            }
                        }
                        item { Text(stringResource(if (permission) R.string.permission_granted else R.string.permission_not_granted), fontSize = 12.sp, color = muted) }
                        item {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text(stringResource(R.string.pause_uploads), modifier = Modifier.weight(1f), fontSize = 12.sp, color = ink)
                                Switch(checked = paused, onCheckedChange = onPauseChanged)
                            }
                        }
                        item { Text(stringResource(R.string.synthetic_notice), fontSize = 11.sp, color = muted, lineHeight = 16.sp) }
                    }
                }
            },
        ) {
            Column(Modifier.fillMaxSize().background(canvas).statusBarsPadding().navigationBarsPadding()) {
                Header(tab, { tab = it }, { scope.launch { drawer.open() } })
                if (tab == 0) {
                    Box(Modifier.weight(1f)) {
                        if (visible.isNotEmpty()) {
                            LazyColumn(Modifier.fillMaxSize().padding(horizontal = 22.dp)) {
                                item { Text("合成事件", modifier = Modifier.padding(top = 26.dp, bottom = 12.dp), fontSize = 13.sp, color = muted) }
                                items(visible, key = { it.eventId }) { event -> EventRow(event) }
                            }
                        } else if (query.isNotBlank()) {
                            Text("没有匹配的合成事件", modifier = Modifier.align(Alignment.Center), fontSize = 13.sp, color = muted)
                        }
                    }
                    SearchDock(query, { query = it }, { scope.launch { drawer.open() } }, onRequestSync)
                } else {
                    Column(Modifier.weight(1f).fillMaxWidth().padding(horizontal = 28.dp, vertical = 32.dp), verticalArrangement = Arrangement.spacedBy(15.dp)) {
                        Text("本机设备", fontSize = 21.sp, fontWeight = FontWeight.SemiBold, color = ink)
                        SelectionContainer {
                            Text(installationId ?: stringResource(R.string.loading), fontSize = 12.sp, color = muted, fontFamily = FontFamily.Monospace)
                        }
                        Text(stringResource(R.string.queue_counts, counts.pending, counts.synced, counts.rejected), fontSize = 13.sp, color = ink)
                        Text("正式设备连接尚未开放。", fontSize = 12.sp, color = muted)
                    }
                }
            }
        }
    }
}

@Composable
private fun DrawerAction(label: String, onClick: () -> Unit) {
    Button(onClick = onClick, colors = ButtonDefaults.buttonColors(containerColor = ink)) { Text(label) }
}

@Composable
private fun Header(tab: Int, onTab: (Int) -> Unit, onMenu: () -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Surface(modifier = Modifier.size(48.dp).clickable(onClick = onMenu).semantics { contentDescription = "打开菜单" }, shape = CircleShape, color = Color.White) {
            Canvas(Modifier.fillMaxSize()) {
                drawLine(ink, Offset(size.width * .30f, size.height * .42f), Offset(size.width * .70f, size.height * .42f), 2.6.dp.toPx())
                drawLine(ink, Offset(size.width * .30f, size.height * .60f), Offset(size.width * .58f, size.height * .60f), 2.6.dp.toPx())
            }
        }
        Row(modifier = Modifier.weight(1f).padding(horizontal = 15.dp).background(segment, RoundedCornerShape(50)).padding(4.dp)) {
            listOf("收件箱", "设备").forEachIndexed { index, title ->
                Box(
                    modifier = Modifier.weight(1f).height(43.dp)
                        .background(if (tab == index) Color.White else Color.Transparent, RoundedCornerShape(50))
                        .clickable { onTab(index) },
                    contentAlignment = Alignment.Center,
                ) { Text(title, fontSize = 15.sp, color = ink) }
            }
        }
        Surface(modifier = Modifier.size(48.dp).clickable(onClick = onMenu).semantics { contentDescription = "打开验证信息" }, shape = CircleShape, color = Color.White) {
            Box(contentAlignment = Alignment.Center) { Text("R", fontSize = 21.sp, fontWeight = FontWeight.SemiBold, color = ink) }
        }
    }
}

@Composable
private fun SearchDock(query: String, onQuery: (String) -> Unit, onAdd: () -> Unit, onSync: () -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 16.dp).shadow(12.dp, RoundedCornerShape(36.dp)),
        color = Color.White,
        shape = RoundedCornerShape(36.dp),
    ) {
        Row(Modifier.fillMaxWidth().height(62.dp).padding(horizontal = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(48.dp).clickable(onClick = onAdd), contentAlignment = Alignment.Center) {
                Text("+", fontSize = 34.sp, fontWeight = FontWeight.Light, color = ink)
            }
            BasicTextField(
                value = query,
                onValueChange = onQuery,
                modifier = Modifier.weight(1f).padding(horizontal = 8.dp),
                singleLine = true,
                textStyle = TextStyle(color = ink, fontSize = 15.sp),
                cursorBrush = SolidColor(ink),
                decorationBox = { inner ->
                    Box {
                        if (query.isEmpty()) Text("搜索合成事件", color = muted, fontSize = 15.sp)
                        inner()
                    }
                },
            )
            Button(
                onClick = onSync,
                modifier = Modifier.size(48.dp).semantics { contentDescription = "请求同步到电脑" },
                shape = CircleShape,
                colors = ButtonDefaults.buttonColors(containerColor = yellow, contentColor = ink),
                contentPadding = PaddingValues(0.dp),
            ) { Text("↻", fontSize = 27.sp) }
        }
    }
}

@Composable
private fun EventRow(event: StoredEvent) {
    Column(Modifier.fillMaxWidth()) {
        Row(Modifier.fillMaxWidth().padding(vertical = 15.dp), verticalAlignment = Alignment.CenterVertically) {
            Surface(modifier = Modifier.size(42.dp), shape = CircleShape, color = segment) {
                Box(contentAlignment = Alignment.Center) {
                    Text(if (event.source == "sms_broadcast") "短" else "测", fontSize = 15.sp, color = ink)
                }
            }
            Spacer(Modifier.width(13.dp))
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                Text(stringResource(if (event.source == "sms_broadcast") R.string.source_sms_broadcast else R.string.source_injected), fontSize = 14.sp, color = ink, fontWeight = FontWeight.Medium)
                Text(event.eventId, fontSize = 11.sp, color = muted, fontFamily = FontFamily.Monospace, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            Text(DateFormat.format("MM-dd", event.receivedAtMs).toString(), fontSize = 10.sp, color = muted)
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(segment))
    }
}
