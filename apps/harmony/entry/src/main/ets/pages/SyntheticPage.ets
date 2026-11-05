/*
 * SyntheticPage — 合成验证（仅开发）。
 *
 * PoC 隔离（FORMAL-UI-MAIL-PLAN §2.1 / 实施任务第 5 条）：原 Index.ets 的合成链路逻辑
 * （配对 apply/finish、synthetic-sync 上传/轮询/确认、pending 队列 preferences、UUID 生成）
 * 原样搬移至此，语义未改；LOCAL_SERVER 常量不动。本页是 PoC 唯一入口，仅从
 * 配置 → 开发 进入，不占正式首页。演示卡片带「演示数据」标记，且不可点入正式详情。
 */
import { common } from '@kit.AbilityKit'
import { preferences } from '@kit.ArkData'
import { cryptoFramework } from '@kit.CryptoArchitectureKit'
import { http } from '@kit.NetworkKit'
import { DetailTopBar } from '../components/DetailTopBar'
import { SmsCard, MailCard } from '../components/RelayCards'
import { SmsCardData, MailCardData } from '../model/RelayModels'

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

@Component
export struct SyntheticPage {
  pathStack: NavPathStack = new NavPathStack()
  @State private invitationID: string = ''
  @State private requestID: string = ''
  @State private claimToken: string = ''
  @State private comparisonCode: string = ''
  @State private deviceID: string = ''
  @State private deviceToken: string = ''
  @State private pendingEventID: string = ''
  @State private pendingAckID: string = ''
  @State private connectionStatus: ResourceStr = $r('app.string.pairing_status_none')
  @State private eventStatus: ResourceStr = $r('app.string.event_status_none')
  @State private pingStatus: ResourceStr = $r('app.string.ping_status_none')
  private demoSmsPrimary: SmsCardData | null = null
  private demoSmsUnconfirmed: SmsCardData | null = null
  private demoMail: MailCardData | null = null

  aboutToAppear(): void {
    this.invitationID = AppStorage.get('relay_invitation_id') as string ?? ''
    this.demoSmsPrimary = {
      deviceName: $r('app.string.demo_sms_device'),
      simLabel: $r('app.string.demo_sms_sim'),
      confirmedLocalNumber: $r('app.string.demo_sms_number'),
      sender: $r('app.string.demo_sms_sender'),
      snippet: $r('app.string.demo_sms_snippet'),
      timeText: $r('app.string.demo_sms_time'),
      statusText: $r('app.string.demo_sms_status'),
      directionText: $r('app.string.sms_direction_received')
    }
    this.demoSmsUnconfirmed = {
      deviceName: $r('app.string.demo_sms_device'),
      simLabel: $r('app.string.demo_sms_sim'),
      confirmedLocalNumber: null,
      sender: $r('app.string.demo_sms_sender_alt'),
      snippet: $r('app.string.demo_sms_snippet_alt'),
      timeText: $r('app.string.demo_sms_time_alt'),
      statusText: $r('app.string.demo_sms_status'),
      directionText: $r('app.string.sms_direction_sent')
    }
    this.demoMail = {
      accountAddress: $r('app.string.demo_mail_account'),
      sender: $r('app.string.demo_mail_sender'),
      subject: $r('app.string.demo_mail_subject'),
      snippet: $r('app.string.demo_mail_snippet'),
      timeText: $r('app.string.demo_mail_time'),
      hasAttachment: true,
      unread: true
    }
    this.restorePending()
  }

  // ---- 以下为原 Index.ets 合成链路逻辑的逐行搬移（仅状态文案改走资源引用） ----

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
        this.eventStatus = $r('app.string.event_status_pending')
      }
      if (this.pendingAckID !== '') {
        this.pingStatus = $r('app.string.ping_status_pending')
      }
    } catch (_error) {
      this.eventStatus = $r('app.string.queue_read_failed')
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
      this.connectionStatus = $r('app.string.pairing_input_incomplete')
      return
    }
    try {
      const text = await this.post('/v1/pairings/' + this.invitationID + '/requests',
        JSON.stringify({ device_label: 'HarmonyOS 模拟器' }))
      const response = JSON.parse(text) as PairingRequestResponse
      this.requestID = response.request_id
      this.claimToken = response.claim_token
      this.comparisonCode = response.comparison_code
      this.connectionStatus = $r('app.string.pairing_request_ok')
    } catch (_error) {
      this.connectionStatus = $r('app.string.pairing_request_failed')
    }
  }

  private async finishPairing(): Promise<void> {
    if (this.claimToken === '' || this.requestID === '') {
      this.connectionStatus = $r('app.string.pairing_need_request')
      return
    }
    try {
      const text = await this.post('/v1/pairings/' + this.invitationID + '/exchange',
        JSON.stringify({ request_id: this.requestID, claim_token: this.claimToken }))
      const response = JSON.parse(text) as DeviceResponse
      this.deviceID = response.device_id
      this.deviceToken = response.device_token
      this.claimToken = ''
      this.connectionStatus = $r('app.string.pairing_done')
    } catch (_error) {
      this.connectionStatus = $r('app.string.pairing_finish_failed')
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
      this.eventStatus = $r('app.string.event_need_pairing')
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
      this.eventStatus = response.event_status === 'accepted'
        ? $r('app.string.event_accepted') : $r('app.string.event_duplicate')
      await this.rememberPing(response)
    } catch (_error) {
      this.eventStatus = $r('app.string.event_send_unconfirmed')
    }
  }

  private async pollSyntheticPing(): Promise<void> {
    if (this.deviceToken === '') {
      this.pingStatus = $r('app.string.event_need_pairing')
      return
    }
    try {
      const text = await this.post('/v1/agent/synthetic-sync', '{}', this.deviceToken)
      await this.rememberPing(JSON.parse(text) as SyncResponse)
    } catch (_error) {
      this.pingStatus = $r('app.string.ping_pull_failed')
    }
  }

  private async rememberPing(response: SyncResponse): Promise<void> {
    if (response.pings.length === 0) {
      if (this.pendingAckID === '') {
        this.pingStatus = $r('app.string.ping_empty')
      }
      return
    }
    const store = await this.localStore()
    this.pendingAckID = response.pings[0].ping_id
    await store.put('pending_ack_id', this.pendingAckID)
    await store.flush()
    this.pingStatus = $r('app.string.ping_received')
  }

  private async acknowledgePing(): Promise<void> {
    if (this.deviceToken === '' || this.pendingAckID === '') {
      this.pingStatus = $r('app.string.ping_need_ack')
      return
    }
    try {
      await this.post('/v1/agent/synthetic-sync',
        JSON.stringify({ ack_ping_id: this.pendingAckID }), this.deviceToken)
      const store = await this.localStore()
      await store.delete('pending_ack_id')
      await store.flush()
      this.pendingAckID = ''
      this.pingStatus = $r('app.string.ping_acked')
    } catch (_error) {
      this.pingStatus = $r('app.string.ping_ack_unconfirmed')
    }
  }

  // ---- UI ----

  @Builder
  actionButton(label: Resource, filled: boolean, onPress: () => void) {
    Row() {
      Text(label)
        .fontSize(13)
        .fontColor(filled ? $r('app.color.relay_accent_text') : $r('app.color.relay_ink'))
    }
    .borderRadius(24)
    .backgroundColor(filled ? $r('app.color.relay_accent') : $r('app.color.relay_segment'))
    .padding({ left: 18, right: 18, top: 10, bottom: 10 })
    .onClick(() => {
      onPress()
    })
    .accessibilityText(label)
  }

  build() {
    NavDestination() {
      Column() {
        Column() {
          DetailTopBar({
            pathStack: this.pathStack,
            title: $r('app.string.synthetic_verification_title')
          })

          Scroll() {
            Column() {
              Text($r('app.string.synthetic_link_notice'))
                .fontSize(21)
                .fontWeight(FontWeight.Bold)
                .fontColor($r('app.color.relay_ink'))
                .width('100%')
              Text($r('app.string.synthetic_page_notice'))
                .fontSize(13)
                .lineHeight(18)
                .fontColor($r('app.color.relay_muted'))
                .width('100%')
                .margin({ top: 10 })

              Column({ space: 12 }) {
                Text($r('app.string.synthetic_panel_title'))
                  .fontSize(15)
                  .fontWeight(FontWeight.Medium)
                  .fontColor($r('app.color.relay_ink'))
                Text(this.eventStatus)
                  .fontSize(14)
                  .fontColor($r('app.color.relay_muted'))
                this.actionButton(
                  this.deviceToken === '' ? $r('app.string.synthetic_upload_hint_unpaired')
                    : $r('app.string.synthetic_upload_button'),
                  true,
                  (): void => {
                    this.sendSyntheticEvent()
                  })

                Blank().height(8)
                Text($r('app.string.pairing_section_title'))
                  .fontSize(17)
                  .fontWeight(FontWeight.Bold)
                  .fontColor($r('app.color.relay_ink'))
                Text($r('app.string.pairing_section_detail'))
                  .fontSize(13)
                  .lineHeight(18)
                  .fontColor($r('app.color.relay_muted'))
                TextInput({ placeholder: $r('app.string.pairing_invitation_hint'), text: this.invitationID })
                  .fontSize(13)
                  .height(48)
                  .fontColor($r('app.color.relay_ink'))
                  .placeholderColor($r('app.color.relay_muted'))
                  .backgroundColor($r('app.color.relay_canvas'))
                  .maxLength(36)
                  .onChange((value: string) => {
                    this.invitationID = value
                  })
                this.actionButton($r('app.string.pairing_apply'), true, (): void => {
                  this.applyPairing()
                })
                if (this.comparisonCode !== '') {
                  Text($r('app.string.pairing_compare_code', this.comparisonCode))
                    .fontSize(21)
                    .fontColor($r('app.color.relay_ink'))
                }
                Text(this.connectionStatus)
                  .fontSize(13)
                  .fontColor($r('app.color.relay_muted'))
                this.actionButton($r('app.string.pairing_finish'), false, (): void => {
                  this.finishPairing()
                })

                Blank().height(14)
                Text($r('app.string.synthetic_ping_section'))
                  .fontSize(17)
                  .fontWeight(FontWeight.Bold)
                  .fontColor($r('app.color.relay_ink'))
                Text(this.pingStatus)
                  .fontSize(13)
                  .fontColor($r('app.color.relay_muted'))
                Row({ space: 12 }) {
                  this.actionButton($r('app.string.synthetic_pull_ping'), false, (): void => {
                    this.pollSyntheticPing()
                  })
                  this.actionButton($r('app.string.synthetic_ack_ping'), true, (): void => {
                    this.acknowledgePing()
                  })
                }
              }
              .alignItems(HorizontalAlign.Start)
              .width('100%')
              .borderRadius(18)
              .backgroundColor($r('app.color.relay_surface'))
              .borderWidth(1)
              .borderColor($r('app.color.relay_border'))
              .padding(16)
              .margin({ top: 18 })

              Text($r('app.string.demo_section_title'))
                .fontSize(13)
                .fontColor($r('app.color.relay_muted'))
                .width('100%')
                .margin({ top: 24, bottom: 6 })
              Text($r('app.string.demo_section_notice'))
                .fontSize(11)
                .lineHeight(16)
                .fontColor($r('app.color.relay_muted'))
                .width('100%')
              Blank().height(12)
              SmsCard({ data: this.demoSmsPrimary, demo: true })
              Blank().height(10)
              SmsCard({ data: this.demoSmsUnconfirmed, demo: true })
              Blank().height(10)
              MailCard({ data: this.demoMail, demo: true })
              Blank().height(28)
            }
            .alignItems(HorizontalAlign.Start)
            .width('100%')
            .padding({ left: 22, right: 22, top: 8, bottom: 8 })
          }
          .width('100%')
          .layoutWeight(1)
          .align(Alignment.TopStart)
          .scrollable(ScrollDirection.Vertical)
        }
        .width('100%')
        .constraintSize({ maxWidth: 780 })
        .height('100%')
      }
      .width('100%')
      .height('100%')
      .alignItems(HorizontalAlign.Center)
      .backgroundColor($r('app.color.relay_canvas'))
    }
    .hideTitleBar(true)
    .backgroundColor($r('app.color.relay_canvas'))
    .onReady((context: NavDestinationContext) => {
      this.pathStack = context.pathStack
    })
  }
}
