import {
  AIFinding,
  AIReport,
  ChartPoint,
  InsightsHeatmapCell,
  InsightsTimelinePoint
} from '../model/DashboardModels';
import {
  COLORS,
  clamp,
  formatNumber,
  formatPercent,
  formatRelative,
  levelLabel,
  scoreColor,
  severityColor,
  statusMeta,
  weekdayName
} from '../utils/Theme';

const HOURS: number[] = buildHours();
const WEEKDAYS: string[] = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];

function buildHours(): number[] {
  const result: number[] = [];
  for (let i = 0; i < 24; i++) {
    result.push(i);
  }
  return result;
}

function alphaColor(hex: string, alpha: number): string {
  const value: number = Math.max(0, Math.min(255, Math.round(alpha * 255)));
  return `#${value.toString(16).padStart(2, '0')}${hex.substring(1)}`;
}

@Styles
function glassCardStyle() {
  .width('100%')
  .padding(16)
  .borderRadius(20)
  .backgroundColor('#CC0B111C')
  .border({ width: 1, color: '#1F94A3B8' })
  .backgroundBlurStyle(BlurStyle.COMPONENT_THICK)
}

@Component
export struct GlassCard {
  @BuilderParam content: () => void;
  @Prop kicker: string = '';
  @Prop title: string = '';
  @Prop subtitle: string = '';

  build() {
    Column({ space: 12 }) {
      if (this.kicker.length > 0 || this.title.length > 0) {
        Column({ space: 4 }) {
          if (this.kicker.length > 0) {
            Text(this.kicker)
              .fontSize(9)
              .fontColor(COLORS.dim)
              .letterSpacing(1.4)
          }
          if (this.title.length > 0) {
            Text(this.title)
              .fontSize(18)
              .fontWeight(FontWeight.Medium)
              .fontColor(COLORS.text)
          }
          if (this.subtitle.length > 0) {
            Text(this.subtitle)
              .fontSize(11)
              .fontColor(COLORS.muted)
              .maxLines(2)
          }
        }
        .alignItems(HorizontalAlign.Start)
        .width('100%')
      }
      this.content()
    }
    .alignItems(HorizontalAlign.Start)
    .glassCardStyle()
  }
}

@Component
export struct TinyChip {
  @Prop label: string = '';
  @Prop color: string = COLORS.slate;

  build() {
    Text(this.label)
      .fontSize(8)
      .fontColor(this.color)
      .padding({ left: 6, right: 6, top: 3, bottom: 3 })
      .margin({ right: 4, bottom: 3 })
      .borderRadius(7)
      .backgroundColor('#0F94A3B8')
  }
}

@Component
export struct MiniStat {
  @Prop label: string = '';
  @Prop value: string = '';
  @Prop hint: string = '';
  @Prop accent: string = COLORS.cyan;

  build() {
    Column({ space: 3 }) {
      Text(this.label)
        .fontSize(8)
        .fontColor(COLORS.dim)
      Text(this.value)
        .fontSize(17)
        .fontWeight(FontWeight.Medium)
        .fontColor(this.accent)
      Text(this.hint)
        .fontSize(7)
        .fontColor(COLORS.dim)
        .maxLines(1)
    }
    .layoutWeight(1)
    .padding({ left: 9, right: 9, top: 8, bottom: 8 })
    .borderRadius(11)
    .backgroundColor('#0F94A3B8')
    .alignItems(HorizontalAlign.Start)
  }
}

@Component
export struct ChartStat {
  @Prop label: string = '';
  @Prop value: string = '';

  build() {
    Column({ space: 2 }) {
      Text(this.label)
        .fontSize(7)
        .fontColor(COLORS.dim)
        .letterSpacing(0.6)
      Text(this.value)
        .fontSize(12)
        .fontColor(COLORS.text)
    }
    .alignItems(HorizontalAlign.Start)
  }
}

@Component
export struct StatusPill {
  @Prop status: string = 'unknown';
  @Prop compact: boolean = false;

  build() {
    Row({ space: 5 }) {
      Circle({ width: 6, height: 6 }).fill(statusMeta(this.status).color)
      Text(statusMeta(this.status).label)
        .fontSize(this.compact ? 8 : 9)
        .fontColor(statusMeta(this.status).color)
        .letterSpacing(0.8)
      if (!this.compact) {
        Text(statusMeta(this.status).zh)
          .fontSize(9)
          .fontColor(COLORS.muted)
      }
    }
    .padding({ left: 9, right: 9, top: 5, bottom: 5 })
    .borderRadius(20)
    .backgroundColor(statusMeta(this.status).soft)
    .border({ width: 1, color: alphaColor(statusMeta(this.status).color, 0.28) })
  }
}

@Component
export struct MetricTile {
  @Prop icon: Resource = $r('sys.symbol.brightness');
  @Prop label: string = '';
  @Prop value: string = '';
  @Prop unit: string = '';
  @Prop hint: string = '';
  @Prop accent: string = '#22D3EE';
  @Prop live: boolean = false;

  build() {
    Column({ space: 7 }) {
      Row({ space: 6 }) {
        SymbolGlyph(this.icon)
          .fontSize(17)
          .fontColor([this.accent])
        Text(this.label)
          .fontSize(10)
          .fontColor(COLORS.muted)
        Blank()
        if (this.live) {
          Text('LIVE')
            .fontSize(7)
            .fontColor(this.accent)
            .letterSpacing(0.8)
            .padding({ left: 4, right: 4, top: 1, bottom: 1 })
            .border({ width: 1, color: alphaColor(this.accent, 0.24) })
            .borderRadius(8)
        }
      }
      .width('100%')

      Row({ space: 3 }) {
        Text(this.value)
          .fontSize(23)
          .fontWeight(FontWeight.Medium)
          .fontColor(COLORS.text)
        if (this.unit.length > 0) {
          Text(this.unit)
            .fontSize(8)
            .fontColor(COLORS.dim)
            .letterSpacing(0.6)
        }
      }
      .alignItems(VerticalAlign.Bottom)

      Text(this.hint)
        .fontSize(8)
        .fontColor(COLORS.dim)
        .maxLines(1)
        .textOverflow({ overflow: TextOverflow.Ellipsis })
    }
    .width('100%')
    .padding(12)
    .borderRadius(16)
    .backgroundColor('#B30D1522')
    .border({ width: 1, color: '#1494A3B8' })
    .alignItems(HorizontalAlign.Start)
  }
}

@Component
export struct Face {
  @Prop status: string = 'unknown';
  @Prop person: boolean = true;
  @State private blink: boolean = false;
  private blinkTimer: number = -1;
  private closeTimer: number = -1;

  aboutToAppear(): void {
    this.blinkTimer = setInterval(() => {
      this.blink = true;
      this.closeTimer = setTimeout(() => {
        this.blink = false;
      }, 120);
    }, 3200);
  }

  aboutToDisappear(): void {
    if (this.blinkTimer >= 0) {
      clearInterval(this.blinkTimer);
      this.blinkTimer = -1;
    }
    if (this.closeTimer >= 0) {
      clearTimeout(this.closeTimer);
      this.closeTimer = -1;
    }
  }

  private mouthPath(): string {
    if (!this.person) {
      return 'M0 6 Q18 3 36 6';
    }
    if (this.status === 'bad') {
      return 'M0 13 Q18 0 36 13';
    }
    if (this.status === 'warning') {
      return 'M0 10 Q18 4 36 10';
    }
    return 'M0 4 Q18 20 36 4';
  }

  build() {
    Stack({ alignContent: Alignment.Center }) {
      Circle({ width: 158, height: 158 })
        .fill('#0B1524')
        .stroke(alphaColor(statusMeta(this.person ? this.status : 'unknown').color, 0.55))
        .strokeWidth(1.4)
      Circle({ width: 136, height: 136 })
        .fill('#00000000')
        .stroke(alphaColor(statusMeta(this.person ? this.status : 'unknown').color, 0.12))
        .strokeWidth(10)
      Column({ space: 12 }) {
        Row({ space: 34 }) {
          Ellipse({ width: 18, height: 30 })
            .fill(statusMeta(this.person ? this.status : 'unknown').color)
            .scale({ x: 1, y: this.blink || !this.person ? 0.12 : 1 })
            .animation({ duration: 120, curve: Curve.EaseInOut })
          Ellipse({ width: 18, height: 30 })
            .fill(statusMeta(this.person ? this.status : 'unknown').color)
            .scale({ x: 1, y: this.blink || !this.person ? 0.12 : 1 })
            .animation({ duration: 120, curve: Curve.EaseInOut })
        }
        Path()
          .commands(this.mouthPath())
          .width(36)
          .height(18)
          .stroke(statusMeta(this.person ? this.status : 'unknown').color)
          .strokeWidth(3)
          .fill(Color.Transparent)
      }
    }
    .width(174)
    .height(174)
  }
}

@Component
export struct SparkLine {
  @Prop points: ChartPoint[] = [];
  @Prop accent: string = '#22D3EE';
  @Prop chartHeight: number = 118;

  private linePath(): string {
    if (this.points.length < 2) {
      return 'M0 50 L100 50';
    }
    let min: number = this.points[0].value;
    let max: number = this.points[0].value;
    for (let i = 1; i < this.points.length; i++) {
      const value: number = this.points[i].value;
      if (value < min) {
        min = value;
      }
      if (value > max) {
        max = value;
      }
    }
    const span: number = max - min === 0 ? 1 : max - min;
    const commands: string[] = [];
    for (let i = 0; i < this.points.length; i++) {
      const x: number = (i / (this.points.length - 1)) * 100;
      const y: number = 88 - ((this.points[i].value - min) / span) * 76;
      commands.push(`${i === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`);
    }
    return commands.join(' ');
  }

  private areaPath(): string {
    return `${this.linePath()} L100 100 L0 100 Z`;
  }

  build() {
    Shape() {
      Path()
        .commands(this.areaPath())
        .fill(alphaColor(this.accent, 0.16))
      Path()
        .commands(this.linePath())
        .stroke(this.accent)
        .strokeWidth(2)
        .fill(Color.Transparent)
    }
    .viewPort({ x: 0, y: 0, width: 100, height: 100 })
    .width('100%')
    .height(this.chartHeight)
  }
}

@Component
export struct Heatmap {
  @Prop cells: InsightsHeatmapCell[] = [];
  @Prop metric: string = 'alert';

  private cellFor(day: number, hour: number): InsightsHeatmapCell | null {
    for (let i = 0; i < this.cells.length; i++) {
      const cell: InsightsHeatmapCell = this.cells[i];
      if (cell.weekday === day && cell.hour === hour) {
        return cell;
      }
    }
    return null;
  }

  private metricValue(cell: InsightsHeatmapCell): number {
    if (this.metric === 'good') {
      return cell.good_rate;
    }
    if (this.metric === 'light') {
      return cell.avg_light / 100;
    }
    return cell.alert_rate;
  }

  private baseColor(): string {
    if (this.metric === 'good') {
      return COLORS.emerald;
    }
    if (this.metric === 'light') {
      return COLORS.amber;
    }
    return COLORS.rose;
  }

  @Builder
  private cell(day: number, hour: number) {
    if (this.cellFor(day, hour) === null) {
      Column()
        .width(14)
        .height(14)
        .borderRadius(3)
        .backgroundColor('#141B28')
    } else {
      Column()
        .width(14)
        .height(14)
        .borderRadius(3)
        .backgroundColor(this.baseColor())
        .opacity(0.18 + clamp(this.metricValue(this.cellFor(day, hour) as InsightsHeatmapCell), 0, 1) * 0.82)
        .onClick(() => {
          const cell: InsightsHeatmapCell = this.cellFor(day, hour) as InsightsHeatmapCell;
          this.getUIContext().getPromptAction().showToast({
            message: `${weekdayName(day)} ${hour}:00 · 良好率 ${formatPercent(cell.good_rate)} · 告警率 ${formatPercent(cell.alert_rate)}`
          });
        })
    }
  }

  build() {
    Scroll() {
      Column({ space: 3 }) {
        Row({ space: 3 }) {
          Text('')
            .width(36)
          ForEach(HOURS, (hour: number) => {
            Text(hour % 3 === 0 ? `${hour}` : '')
              .width(14)
              .fontSize(7)
              .fontColor(COLORS.dim)
              .textAlign(TextAlign.Center)
          }, (hour: number) => `h-${hour}`)
        }
        .width(447)

        ForEach(WEEKDAYS, (day: string, rowIndex: number) => {
          Row({ space: 3 }) {
            Text(day)
              .width(36)
              .fontSize(8)
              .fontColor(COLORS.dim)
            ForEach(HOURS, (hour: number) => {
              this.cell(rowIndex + 1, hour)
            }, (hour: number) => `c-${rowIndex}-${hour}`)
          }
          .width(447)
        }, (day: string) => day)
      }
      .width(447)
      .padding({ top: 2, bottom: 2 })
    }
    .scrollable(ScrollDirection.Horizontal)
    .scrollBar(BarState.Off)
    .width('100%')
  }
}

@Component
export struct TimelineStrip {
  @Prop timeline: InsightsTimelinePoint[] = [];
  @Prop activeIndex: number = -1;
  onSelect: (index: number) => void = () => {
  };

  build() {
    Row({ space: 1 }) {
      ForEach(this.timeline, (point: InsightsTimelinePoint, index: number) => {
        Column()
          .layoutWeight(1)
          .height(8)
          .borderRadius(1)
          .backgroundColor(statusMeta(point.status).color)
          .opacity(index === this.activeIndex ? 1 : 0.42 + (index / Math.max(1, this.timeline.length)) * 0.35)
          .onClick(() => this.onSelect(index))
      }, (point: InsightsTimelinePoint, index: number) => `${point.time}-${index}`)
    }
    .width('100%')
    .height(8)
  }
}

@Component
export struct AIReportView {
  @Prop report: AIReport;
  onRegenerate: () => void = () => {
  };

  @Builder
  private findingRow(finding: AIFinding) {
    Row({ space: 9 }) {
      Column()
        .width(3)
        .height(32)
        .borderRadius(2)
        .backgroundColor(severityColor(finding.severity))
      Column({ space: 3 }) {
        Text(finding.title)
          .fontSize(12)
          .fontWeight(FontWeight.Medium)
          .fontColor(COLORS.text)
        Text(finding.detail)
          .fontSize(10)
          .fontColor(COLORS.muted)
          .lineHeight(16)
      }
      .layoutWeight(1)
      .alignItems(HorizontalAlign.Start)
    }
    .width('100%')
    .alignItems(VerticalAlign.Top)
    .padding({ top: 7, bottom: 7 })
  }

  build() {
    Column({ space: 12 }) {
      Row({ space: 12 }) {
        Stack({ alignContent: Alignment.Center }) {
          Progress({ value: this.report.score, total: 100, type: ProgressType.Ring })
            .width(74)
            .height(74)
            .color(scoreColor(this.report.score))
            .backgroundColor('#1A94A3B8')
            .style({ strokeWidth: 7 })
          Text(`${this.report.score}`)
            .fontSize(24)
            .fontWeight(FontWeight.Medium)
            .fontColor(scoreColor(this.report.score))
        }
        .width(78)
        .height(78)

        Column({ space: 4 }) {
          Row({ space: 6 }) {
            Text(levelLabel(this.report.level))
              .fontSize(9)
              .fontColor(COLORS.violet)
              .padding({ left: 6, right: 6, top: 2, bottom: 2 })
              .border({ width: 1, color: '#33A78BFA' })
              .borderRadius(9)
            Text(`${this.report.hours}H WINDOW`)
              .fontSize(8)
              .fontColor(COLORS.dim)
          }
          Text(this.report.headline)
            .fontSize(16)
            .fontWeight(FontWeight.Medium)
            .fontColor(COLORS.text)
            .maxLines(2)
          Text(this.report.summary)
            .fontSize(10)
            .fontColor(COLORS.muted)
            .lineHeight(16)
            .maxLines(3)
            .textOverflow({ overflow: TextOverflow.Ellipsis })
        }
        .layoutWeight(1)
        .alignItems(HorizontalAlign.Start)
      }
      .width('100%')
      .alignItems(VerticalAlign.Top)
      .padding(12)
      .borderRadius(14)
      .backgroundColor('#14A78BFA')

      if (this.report.findings.length > 0) {
        Column({ space: 0 }) {
          Text('关键发现')
            .fontSize(11)
            .fontColor(COLORS.cyan)
            .letterSpacing(0.8)
          ForEach(this.report.findings, (finding: AIFinding) => {
            this.findingRow(finding)
          }, (finding: AIFinding, index: number) => `${finding.title}-${index}`)
        }
        .width('100%')
        .alignItems(HorizontalAlign.Start)
      }

      if (this.report.risk_windows.length > 0) {
        Column({ space: 6 }) {
          Text('高风险时段')
            .fontSize(11)
            .fontColor(COLORS.amber)
            .letterSpacing(0.8)
          Flex({ wrap: FlexWrap.Wrap }) {
            ForEach(this.report.risk_windows, (window: string) => {
              Text(window)
                .fontSize(9)
                .fontColor(COLORS.amber)
                .padding({ left: 7, right: 7, top: 4, bottom: 4 })
                .margin({ right: 5, bottom: 5 })
                .borderRadius(10)
                .backgroundColor('#14FBBF24')
                .border({ width: 1, color: '#26FBBF24' })
            }, (window: string) => window)
          }
        }
        .width('100%')
        .alignItems(HorizontalAlign.Start)
      }

      if (this.report.suggestions.length > 0) {
        Column({ space: 7 }) {
          Text('给家长的建议')
            .fontSize(11)
            .fontColor(COLORS.emerald)
            .letterSpacing(0.8)
          ForEach(this.report.suggestions, (suggestion: string, index: number) => {
            Row({ space: 8 }) {
              Text(`${index + 1}`)
                .fontSize(8)
                .fontColor(COLORS.emerald)
                .width(18)
                .height(18)
                .textAlign(TextAlign.Center)
                .border({ width: 1, color: '#3334D399' })
                .borderRadius(9)
              Text(suggestion)
                .fontSize(10)
                .fontColor(COLORS.muted)
                .lineHeight(16)
                .layoutWeight(1)
            }
            .width('100%')
            .alignItems(VerticalAlign.Top)
          }, (suggestion: string, index: number) => `${suggestion}-${index}`)
        }
        .width('100%')
        .alignItems(HorizontalAlign.Start)
      }

      if (this.report.encouragement.length > 0) {
        Row({ space: 7 }) {
          SymbolGlyph($r('sys.symbol.heart_badge_plus'))
            .fontSize(15)
            .fontColor([COLORS.rose])
          Text(this.report.encouragement)
            .fontSize(10)
            .fontColor('#FECDD3')
            .layoutWeight(1)
        }
        .width('100%')
        .padding(10)
        .borderRadius(10)
        .backgroundColor('#14FB7185')
        .border({ width: { left: 2 }, color: '#80FB7185' })
      }

      Text(this.report.disclaimer)
        .fontSize(8)
        .fontColor(COLORS.dim)
        .lineHeight(13)

      Row({ space: 8 }) {
        Text(`${this.report.source === 'ai' ? 'AI' : '本地规则'} · ${this.report.model}`)
          .fontSize(8)
          .fontColor(COLORS.dim)
        Blank()
        Button('重新分析')
          .fontSize(10)
          .fontColor(COLORS.cyan)
          .height(30)
          .padding({ left: 12, right: 12 })
          .backgroundColor('#1422D3EE')
          .borderRadius(15)
          .onClick(() => this.onRegenerate())
      }
      .width('100%')
    }
    .width('100%')
    .alignItems(HorizontalAlign.Start)
  }
}
