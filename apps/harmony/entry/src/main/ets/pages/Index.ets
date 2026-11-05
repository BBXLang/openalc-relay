import { common } from '@kit.AbilityKit'
import { preferences } from '@kit.ArkData'
import { cryptoFramework } from '@kit.CryptoArchitectureKit'
import { http } from '@kit.NetworkKit'

const CANVAS: string = '#FAFAFA'
const SURFACE: string = '#FFFFFF'
const INK: string = '#111111'
const MUTED: string = '#838383'
const SEGMENT: string = '#E9E9E9'
const YELLOW: string = '#FFD12E'
const LOCAL_SERVER: string = 'http://127.0.0.1:18080'

interface PairingRequestResponse {
  request_id: string
  claim_token: string
  comparison_code: string
}

interface DeviceResponse {
  device_id: string
  device_token: string
}

interface PendingPing {
  ping_id: string
}

interface SyncResponse {
  event_status?: string
  pings: PendingPing[]
}

@Entry
@Component
struct Index {
  @State selectedTab: string = 'inbox'
  @State showInfo: boolean = false
  @State invitationID: string = ''
  @State requestID: string = ''
  @State claimToken: string = ''
  @State comparisonCode: string = ''
  @State deviceID: string = ''
  @State deviceToken: string = ''
  @State pendingEventID: string = ''
  @State pendingAckID: string = ''
  @State connectionStatus: string = '未配对'
  @State eventStatus: string = '尚无合成事件'
  @State pingStatus: string = '尚无服务端指令'

  aboutToAppear(): void {
    this.invitationID = AppStorage.get('relay_invitation_id') as string ?? ''
    this.restorePending()
  }

  private async localStore(): Promise<preferences.Preferences> {
    const context = getContext(this) as common.UIAbilityContext
    return await preferences.getPreferences(context, 'relay_synthetic_queue')
  }

  private async restorePending(): Promise<void> {
    try {
      const store = await this.localStore()
      this.pendingEventID = (await store.get('pending_event_id', '')) as string
      this.pendingAckID = (await store.get('pending_ack_id', '')) as string
      if (this.pendingEventID !== '') {
        this.eventStatus = '本地有待确认的合成事件'
      }
      if (this.pendingAckID !== '') {
        this.pingStatus = '服务端指令待回执'
      }
    } catch (_error) {
      this.eventStatus = '本地队列读取失败'
    }
  }

  private async post(path: string, body: string, credential: string = ''): Promise<string> {
    const request = http.createHttp()
    try {
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      if (credential !== '') {
        headers['Authorization'] = 'Bearer ' + credential
      }
      const response = await request.request(LOCAL_SERVER + path, {
        method: http.RequestMethod.POST,
        body: body,
        header: headers,
        expectDataType: http.HttpDataType.STRING,
        usingCache: false,
        connectTimeout: 5000,
        readTimeout: 5000
      })
      if (response.responseCode < 200 || response.responseCode >= 300) {
        throw new Error('HTTP ' + response.responseCode)
      }
      return String(response.result)
    } finally {
      request.destroy()
    }
  }

  private async applyPairing(): Promise<void> {
    if (this.invitationID.length !== 36) {
      this.connectionStatus = '请输入完整的配对编号'
      return
    }
    try {
      const text = await this.post('/v1/pairings/' + this.invitationID + '/requests',
        JSON.stringify({ device_label: 'HarmonyOS 模拟器' }))
      const response = JSON.parse(text) as PairingRequestResponse
      this.requestID = response.request_id
      this.claimToken = response.claim_token
      this.comparisonCode = response.comparison_code
      this.connectionStatus = '请在管理员端核对验证码并批准'
    } catch (_error) {
      this.connectionStatus = '配对申请失败，请检查本机隧道或邀请有效期'
    }
  }

  private async finishPairing(): Promise<void> {
    if (this.claimToken === '' || this.requestID === '') {
      this.connectionStatus = '请先提交配对申请'
      return
    }
    try {
      const text = await this.post('/v1/pairings/' + this.invitationID + '/exchange',
        JSON.stringify({ request_id: this.requestID, claim_token: this.claimToken }))
      const response = JSON.parse(text) as DeviceResponse
      this.deviceID = response.device_id
      this.deviceToken = response.device_token
      this.claimToken = ''
      this.connectionStatus = '已配对，仅限本次模拟器进程'
    } catch (_error) {
      this.connectionStatus = '尚未批准或配对已过期，请在管理员端检查'
    }
  }

  private newEventID(): string {
    const raw = cryptoFramework.createRandom().generateRandomSync(16).data
    raw[6] = (raw[6] & 0x0F) | 0x40
    raw[8] = (raw[8] & 0x3F) | 0x80
    let hex: string = ''
    for (let i = 0; i < raw.length; i++) {
      hex += raw[i].toString(16).padStart(2, '0')
    }
    return hex.substring(0, 8) + '-' + hex.substring(8, 12) + '-' + hex.substring(12, 16) + '-' +
      hex.substring(16, 20) + '-' + hex.substring(20)
  }

  private async sendSyntheticEvent(): Promise<void> {
    if (this.deviceToken === '') {
      this.eventStatus = '请先完成模拟器配对'
      return
    }
    try {
      const store = await this.localStore()
      if (this.pendingEventID === '') {
        this.pendingEventID = this.newEventID()
        await store.put('pending_event_id', this.pendingEventID)
        await store.flush()
      }
      const text = await this.post('/v1/agent/synthetic-sync',
        JSON.stringify({ client_event_id: this.pendingEventID }), this.deviceToken)
      const response = JSON.parse(text) as SyncResponse
      if (response.event_status !== 'accepted' && response.event_status !== 'duplicate') {
        throw new Error('unexpected acknowledgement')
      }
      await store.delete('pending_event_id')
      await store.flush()
      this.pendingEventID = ''
      this.eventStatus = response.event_status === 'accepted' ? '合成事件已提交' : '重试去重已确认'
      await this.rememberPing(response)
    } catch (_error) {
      this.eventStatus = '发送未确认，合成事件留在本地待重试'
    }
  }

  private async pollSyntheticPing(): Promise<void> {
    if (this.deviceToken === '') {
      this.pingStatus = '请先完成模拟器配对'
      return
    }
    try {
      const text = await this.post('/v1/agent/synthetic-sync', '{}', this.deviceToken)
      await this.rememberPing(JSON.parse(text) as SyncResponse)
    } catch (_error) {
      this.pingStatus = '拉取失败，请检查连接或设备权限'
    }
  }

  private async rememberPing(response: SyncResponse): Promise<void> {
    if (response.pings.length === 0) {
      if (this.pendingAckID === '') {
        this.pingStatus = '暂无待接收的服务端指令'
      }
      return
    }
    const store = await this.localStore()
    this.pendingAckID = response.pings[0].ping_id
    await store.put('pending_ack_id', this.pendingAckID)
    await store.flush()
    this.pingStatus = '已收到服务端合成 ping，等待手动回执'
  }

  private async acknowledgePing(): Promise<void> {
    if (this.deviceToken === '' || this.pendingAckID === '') {
      this.pingStatus = '没有需要回执的服务端指令'
      return
    }
    try {
      await this.post('/v1/agent/synthetic-sync',
        JSON.stringify({ ack_ping_id: this.pendingAckID }), this.deviceToken)
      const store = await this.localStore()
      await store.delete('pending_ack_id')
      await store.flush()
      this.pendingAckID = ''
      this.pingStatus = '服务端合成 ping 已回执'
    } catch (_error) {
      this.pingStatus = '回执未确认，稍后可用同一编号重试'
    }
  }

  build() {
    Column() {
      Row() {
        Text('☰')
          .fontSize(23)
          .fontColor(INK)
          .width(48)
          .height(48)
          .textAlign(TextAlign.Center)
          .backgroundColor(SURFACE)
          .borderRadius(24)
          .onClick(() => { this.showInfo = !this.showInfo })

        Row() {
          Text('收件箱')
            .fontSize(15)
            .fontColor(INK)
            .width('50%')
            .height(42)
            .textAlign(TextAlign.Center)
            .backgroundColor(this.selectedTab === 'inbox' ? SURFACE : SEGMENT)
            .borderRadius(24)
            .onClick(() => { this.selectedTab = 'inbox'; this.showInfo = false })
          Text('设备')
            .fontSize(15)
            .fontColor(INK)
            .width('50%')
            .height(42)
            .textAlign(TextAlign.Center)
            .backgroundColor(this.selectedTab === 'devices' ? SURFACE : SEGMENT)
            .borderRadius(24)
            .onClick(() => { this.selectedTab = 'devices'; this.showInfo = false })
        }
        .layoutWeight(1)
        .height(50)
        .padding(4)
        .margin({ left: 14, right: 14 })
        .backgroundColor(SEGMENT)
        .borderRadius(25)

        Text('R')
          .fontSize(22)
          .fontColor(INK)
          .width(48)
          .height(48)
          .textAlign(TextAlign.Center)
          .backgroundColor(SURFACE)
          .borderRadius(24)
          .onClick(() => { this.showInfo = !this.showInfo })
      }
      .width('100%')
      .constraintSize({ maxWidth: 780 })
      .padding({ left: 16, right: 16, top: 18, bottom: 18 })

      Column() {
        if (this.showInfo) {
          Text('Relay')
            .fontSize(23)
            .fontWeight(FontWeight.Bold)
            .fontColor(INK)
          Text('HarmonyOS 合成链路验证')
            .fontSize(13)
            .fontColor(MUTED)
            .margin({ top: 18 })
          Text('仅连接本机回环测试服务。没有短信、号码、正式发送或推送。')
            .fontSize(13)
            .fontColor(MUTED)
            .margin({ top: 12 })
        } else if (this.selectedTab === 'devices') {
          Text('模拟器配对')
            .fontSize(21)
            .fontWeight(FontWeight.Bold)
            .fontColor(INK)
          Text('输入管理员创建的 5 分钟配对编号。凭证仅保存在本次进程内。')
            .fontSize(13)
            .fontColor(MUTED)
            .margin({ top: 12 })
          TextInput({ placeholder: '配对编号', text: this.invitationID })
            .fontSize(13)
            .height(48)
            .margin({ top: 18 })
            .backgroundColor(SURFACE)
            .onChange((value: string) => { this.invitationID = value })
          Button('申请配对')
            .backgroundColor(YELLOW)
            .fontColor(INK)
            .margin({ top: 12 })
            .onClick(() => { this.applyPairing() })
          if (this.comparisonCode !== '') {
            Text('核对码：' + this.comparisonCode)
              .fontSize(21)
              .fontColor(INK)
              .margin({ top: 16 })
          }
          Text(this.connectionStatus)
            .fontSize(13)
            .fontColor(MUTED)
            .margin({ top: 12 })
          Button('批准后完成配对')
            .backgroundColor(SEGMENT)
            .fontColor(INK)
            .margin({ top: 12 })
            .onClick(() => { this.finishPairing() })
          Text('合成服务端指令')
            .fontSize(17)
            .fontWeight(FontWeight.Bold)
            .fontColor(INK)
            .margin({ top: 30 })
          Text(this.pingStatus)
            .fontSize(13)
            .fontColor(MUTED)
            .margin({ top: 10 })
          Row() {
            Button('拉取 ping')
              .backgroundColor(SEGMENT)
              .fontColor(INK)
              .onClick(() => { this.pollSyntheticPing() })
            Button('确认并回执')
              .backgroundColor(YELLOW)
              .fontColor(INK)
              .margin({ left: 12 })
              .onClick(() => { this.acknowledgePing() })
          }
          .margin({ top: 14 })
        } else {
          Text('合成验证')
            .fontSize(21)
            .fontWeight(FontWeight.Bold)
            .fontColor(INK)
          Text(this.eventStatus)
            .fontSize(14)
            .fontColor(MUTED)
            .margin({ top: 12 })
          Text(this.pingStatus)
            .fontSize(14)
            .fontColor(MUTED)
            .margin({ top: 8 })
        }
      }
      .alignItems(HorizontalAlign.Start)
      .width('100%')
      .constraintSize({ maxWidth: 780 })
      .layoutWeight(1)
      .padding({ left: 28, right: 28, top: 24 })

      if (this.selectedTab === 'inbox') {
        Row() {
          Text('+')
            .fontSize(34)
            .fontColor(INK)
            .width(48)
            .textAlign(TextAlign.Center)
            .onClick(() => { this.showInfo = !this.showInfo })
          Text(this.deviceToken === '' ? '请先在设备页配对' : '上传固定合成事件')
            .fontSize(15)
            .fontColor(MUTED)
            .layoutWeight(1)
            .margin({ left: 10 })
          Text('↑')
            .fontSize(22)
            .fontColor(INK)
            .width(48)
            .height(48)
            .textAlign(TextAlign.Center)
            .backgroundColor(YELLOW)
            .borderRadius(24)
            .onClick(() => { this.sendSyntheticEvent() })
        }
        .width('90%')
        .constraintSize({ maxWidth: 740 })
        .height(64)
        .padding({ left: 8, right: 8 })
        .margin({ bottom: 18 })
        .backgroundColor(SURFACE)
        .borderRadius(32)
      }
    }
    .width('100%')
    .height('100%')
    .alignItems(HorizontalAlign.Center)
    .backgroundColor(CANVAS)
  }
}
