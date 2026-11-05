/*
 * CircleIconButton — 顶栏与详情页的圆形控件（48x48，表面色底）。
 * label 用于无障碍名称（FORMAL-UI-MAIL-PLAN §2.2：右侧圆形搜索钮带无障碍标注）。
 */
@Component
export struct CircleIconButton {
  @BuilderParam content: () => void = this.empty
  onPress: () => void = () => {
  }
  label: Resource = $r('app.string.menu_open')
  enabledButton: boolean = true

  @Builder
  empty() {
  }

  build() {
    Column() {
      this.content()
    }
    .width(48)
    .height(48)
    .borderRadius(24)
    .backgroundColor($r('app.color.relay_surface'))
    .justifyContent(FlexAlign.Center)
    .alignItems(HorizontalAlign.Center)
    .onClick(() => {
      if (this.enabledButton) {
        this.onPress()
      }
    })
    .enabled(this.enabledButton)
    .accessibilityText(this.label)
  }
}
