/*
 * StateBlock — 列表/详情统一状态块（加载、空态、断网、失败、未接通）。
 * 正式页面必须诚实说明原因，绝不渲染伪造会话（FORMAL-UI-MAIL-PLAN §2.2）。
 */
@Component
export struct StateBlock {
  kind: string = 'empty'
  message: ResourceStr = $r('app.string.formal_sync_not_connected')
  detail: ResourceStr | null = null

  build() {
    Column({ space: 10 }) {
      Text(this.kind === 'loading' ? '…' : this.kind === 'offline' ? '⌁' : this.kind === 'failed' ? '!' : '—')
        .fontSize(22)
        .fontColor($r('app.color.relay_muted'))
      Text(this.message)
        .fontSize(14)
        .lineHeight(20)
        .fontColor($r('app.color.relay_ink'))
        .textAlign(TextAlign.Center)
      if (this.detail !== null) {
        Text(this.detail)
          .fontSize(12)
          .lineHeight(17)
          .fontColor($r('app.color.relay_muted'))
          .textAlign(TextAlign.Center)
      }
    }
    .alignItems(HorizontalAlign.Center)
    .padding(28)
  }
}
