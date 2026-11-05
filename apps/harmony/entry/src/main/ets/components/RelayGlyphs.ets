/*
 * RelayGlyphs — 用形状与描边组件拼出的极简图标（对齐 Android Canvas 字形）。
 * 颜色一律接收 ResourceColor，保证暗色模式跟随资源自动切换。
 */
@Component
export struct MenuGlyph {
  color: ResourceColor = $r('app.color.relay_ink')

  build() {
    Column({ space: 5 }) {
      Row()
        .width(19)
        .height(2.4)
        .borderRadius(2)
        .backgroundColor(this.color)
      Row()
        .width(13)
        .height(2.4)
        .borderRadius(2)
        .backgroundColor(this.color)
    }
    .alignItems(HorizontalAlign.Start)
    .justifyContent(FlexAlign.Center)
  }
}

@Component
export struct SearchGlyph {
  color: ResourceColor = $r('app.color.relay_ink')

  build() {
    Stack() {
      Column()
        .width(13)
        .height(13)
        .borderRadius(7)
        .borderWidth(2.2)
        .borderColor(this.color)
        .translate({ x: -2, y: -2 })
      Row()
        .width(9)
        .height(2.2)
        .borderRadius(2)
        .backgroundColor(this.color)
        .rotate({ angle: 45 })
        .translate({ x: 4, y: 4 })
    }
    .width(24)
    .height(24)
    .alignContent(Alignment.Center)
  }
}

@Component
export struct BackChevronGlyph {
  color: ResourceColor = $r('app.color.relay_ink')

  build() {
    Stack() {
      Row()
        .width(13)
        .height(2.4)
        .borderRadius(2)
        .backgroundColor(this.color)
        .rotate({ angle: -45 })
        .translate({ x: 4, y: -4 })
      Row()
        .width(13)
        .height(2.4)
        .borderRadius(2)
        .backgroundColor(this.color)
        .rotate({ angle: 45 })
        .translate({ x: 4, y: 4 })
    }
    .width(24)
    .height(24)
    .alignContent(Alignment.Center)
  }
}

@Component
export struct DeviceSimGlyph {
  color: ResourceColor = $r('app.color.relay_ink')

  build() {
    Stack({ alignContent: Alignment.TopStart }) {
      Column()
        .width(13)
        .height(17)
        .borderRadius(5)
        .borderWidth(1.8)
        .borderColor(this.color)
      Column()
        .width(10)
        .height(12)
        .borderRadius(2)
        .borderWidth(1.8)
        .borderColor(this.color)
        .translate({ x: 8, y: 6 })
    }
    .width(22)
    .height(22)
  }
}

@Component
export struct EnvelopeGlyph {
  color: ResourceColor = $r('app.color.relay_ink')

  build() {
    Stack() {
      Column()
        .width(19)
        .height(13)
        .borderRadius(3)
        .borderWidth(1.8)
        .borderColor(this.color)
      Row()
        .width(14)
        .height(1.8)
        .backgroundColor(this.color)
        .rotate({ angle: -28 })
        .translate({ x: -7, y: -2 })
    }
    .width(24)
    .height(24)
    .alignContent(Alignment.Center)
  }
}

@Component
export struct SendGlyph {
  color: ResourceColor = $r('app.color.relay_accent_text')

  build() {
    Column()
      .width(0)
      .height(0)
      .border({
        width: { left: 9, top: 6, bottom: 6 },
        color: {
          left: this.color,
          top: Color.Transparent,
          bottom: Color.Transparent
        },
        style: { left: BorderStyle.Solid, top: BorderStyle.Solid, bottom: BorderStyle.Solid }
      })
  }
}

/** 展开指示符（▾ / ▸），与 Web 侧栏一致使用文字符号。 */
@Component
export struct DrawerCaret {
  expanded: boolean = false
  color: ResourceColor = $r('app.color.relay_ink')

  build() {
    Text(this.expanded ? '▾' : '▸')
      .fontSize(12)
      .fontColor(this.color)
      .width(18)
      .textAlign(TextAlign.Center)
  }
}
