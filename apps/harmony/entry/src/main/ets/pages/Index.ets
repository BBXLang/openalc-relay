const CANVAS: string = '#FAFAFA'
const SURFACE: string = '#FFFFFF'
const INK: string = '#111111'
const MUTED: string = '#838383'
const SEGMENT: string = '#E9E9E9'
const YELLOW: string = '#FFD12E'

@Entry
@Component
struct Index {
  @State selectedTab: string = 'inbox'
  @State showInfo: boolean = false

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
          Text('HarmonyOS 界面预览')
            .fontSize(13)
            .fontColor(MUTED)
            .margin({ top: 18 })
          Text('尚未连接服务、短信、号码或推送。当前页面只展示统一界面结构。')
            .fontSize(13)
            .fontColor(MUTED)
            .margin({ top: 12 })
        } else if (this.selectedTab === 'devices') {
          Text('本机设备')
            .fontSize(21)
            .fontWeight(FontWeight.Bold)
            .fontColor(INK)
          Text('正式设备连接尚未开放。')
            .fontSize(13)
            .fontColor(MUTED)
            .margin({ top: 12 })
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
          Text('尚未连接服务')
            .fontSize(15)
            .fontColor(MUTED)
            .layoutWeight(1)
            .margin({ left: 10 })
          Text('—')
            .fontSize(22)
            .fontColor(INK)
            .width(48)
            .height(48)
            .textAlign(TextAlign.Center)
            .backgroundColor(YELLOW)
            .borderRadius(24)
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
