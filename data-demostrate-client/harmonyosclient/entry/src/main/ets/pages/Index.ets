import { common } from '@kit.AbilityKit';
import { promptAction, uiMaterial } from '@kit.ArkUI';
import { BusinessError } from '@kit.BasicServicesKit';
import { image } from '@kit.ImageKit';
import { vibrator } from '@kit.SensorServiceKit';
import {
  HdsNavigation,
  HdsNavigationMenuContentOptions,
  HdsNavigationMenuItemOptions,
  HdsNavigationTitleMode,
  HdsTabs,
  HdsTabsController,
  IconStyleMode,
  ScrollEffectType,
  SystemMaterialParams,
  TitleBarContentOptions,
  hdsMaterial
} from '@kit.UIDesignKit';

import {
  AIReport,
  ChartPoint,
  Device,
  DemoResult,
  EventRow,
  Insights,
  InsightsHeatmapCell,
  InsightsRiskWindow,
  InsightsSummary,
  InsightsTimelinePoint,
  Message,
  PostureFrame,
  RangeParams,
  SeriesPoint,
  TelemetryRow
} from '../model/DashboardModels';
import * as Api from '../service/ApiService';
import {
  COLORS,
  clamp,
  formatClock,
  formatDateTime,
  formatNumber,
  formatPercent,
  formatRelative,
  levelLabel,
  scoreColor,
  statusMeta,
  weekdayName
} from '../utils/Theme';
import {
  AIReportView,
  ChartStat,
  Face,
  GlassCard,
  Heatmap,
  MetricTile,
  MiniStat,
  SparkLine,
  StatusPill,
  TimelineStrip,
  TinyChip
} from './../view/DashboardWidgets';

const TITLE_BAR_PLACEHOLDER: number = 98;
const TAB_BOTTOM_PADDING: number = 112;
const IMMERSIVE_MATERIAL: SystemMaterialParams = {
  materialType: hdsMaterial.MaterialType.IMMERSIVE,
  materialLevel: hdsMaterial.MaterialLevel.EXQUISITE
};
const SHEET_MATERIAL: uiMaterial.Material | undefined = createSheetMaterial();

enum AppTab {
  OVERVIEW = 0,
  REPORT = 1,
  MESSAGES = 2,
  REPLAY = 3,
  DEMO = 4
}

interface MixItem {
  key: string;
  label: string;
  count: number;
  color: string;
}

interface DemoScenario {
  key: string;
  label: string;
  hint: string;
  icon: Resource;
  color: string;
}

const DEMO_SCENARIOS: DemoScenario[] = [
  {
    key: 'good',
    label: '良好坐姿',
    hint: '绿色 · 无提醒',
    icon: $r('sys.symbol.checkmark'),
    color: COLORS.emerald
  },
  {
    key: 'warning',
    label: '轻微偏移',
    hint: '黄色 · 一次提醒',
    icon: $r('sys.symbol.exclamationmark_circle'),
    color: COLORS.amber
  },
  {
    key: 'bad',
    label: '坐姿告警',
    hint: '红色 · 强提醒',
    icon: $r('sys.symbol.exclamationmark_circle_fill'),
    color: COLORS.rose
  },
  {
    key: 'away',
    label: '离开书桌',
    hint: '灰色 · 无人',
    icon: $r('sys.symbol.person_fill'),
    color: COLORS.slate
  }
];

const RANGES: string[] = ['1H', '6H', '24H', '7D'];

function createSheetMaterial(): uiMaterial.Material | undefined {
  try {
    return new uiMaterial.ImmersiveMaterial({ style: uiMaterial.ImmersiveStyle.THICK });
  } catch (error) {
    return undefined;
  }
}

function resolveMaterialParams(): SystemMaterialParams {
  try {
    const types: Array<hdsMaterial.MaterialType> = hdsMaterial.getSystemMaterialTypes();
    if (types.indexOf(hdsMaterial.MaterialType.IMMERSIVE) >= 0) {
      return IMMERSIVE_MATERIAL;
    }
  } catch (error) {
    // Fall through to the non-material style.
  }
  return {
    materialType: hdsMaterial.MaterialType.NONE,
    materialLevel: hdsMaterial.MaterialLevel.SMOOTH
  };
}

function statusOf(value: string): string {
  if (value === 'good' || value === 'warning' || value === 'bad') {
    return value;
  }
  return 'unknown';
}

function messageOf(error: Object): string {
  if (error instanceof Error) {
    return error.message;
  }
  return `${error}`;
}

@Entry
@Component
struct Index {
  private tabsController: HdsTabsController = new HdsTabsController();
  private pathStack: NavPathStack = new NavPathStack();
  private materialParams: SystemMaterialParams = resolveMaterialParams();
  private refreshTimer: number = -1;
  private replayTimer: number = -1;
  private demoTimer: number = -1;
  private demoIndex: number = 0;
  private inFlight: boolean = false;
  private autoReportDeviceId: string = '';
  private loadedFrameId: number = -1;

  @StorageLink('apiBaseUrl') apiBaseUrl: string = Api.DEFAULT_BASE_URL;
  @StorageLink('apiToken') apiToken: string = Api.DEFAULT_API_TOKEN;
  @State activeTab: number = AppTab.OVERVIEW;
  @State devices: Device[] = [];
  @State selectedDeviceIndex: number = 0;
  @State selectedDeviceId: string = '';
  @State latest: TelemetryRow[] = [];
  @State lightPoints: ChartPoint[] = [];
  @State rssiPoints: ChartPoint[] = [];
  @State heapPoints: ChartPoint[] = [];
  @State postureRows: TelemetryRow[] = [];
  @State events: EventRow[] = [];
  @State insights: Insights | null = null;
  @State frames: PostureFrame[] = [];
  @State selectedFrameId: number = 0;
  @State framePixel: image.PixelMap | undefined = undefined;
  @State frameLoading: boolean = false;
  @State frameError: string = '';
  @State aiReport: AIReport | null = null;
  @State aiLoading: boolean = false;
  @State aiError: string = '';
  @State messages: Message[] = [];
  @State messageDraft: string = '';
  @State messageRole: string = 'parent';
  @State messageSending: boolean = false;
  @State messageError: string = '';
  @State loading: boolean = true;
  @State refreshing: boolean = false;
  @State errorMessage: string = '';
  @State lastUpdated: string = '';
  @State activeRange: string = '24H';
  @State autoRefresh: boolean = true;
  @State replayActive: boolean = false;
  @State replayPlaying: boolean = false;
  @State replayIndex: number = 0;
  @State heatMetric: string = 'alert';
  @State showSettings: boolean = false;
  @State editBaseUrl: string = '';
  @State editToken: string = '';
  @State demoBusy: boolean = false;
  @State demoMessage: string = '';
  @State autoDemo: boolean = false;

  aboutToAppear(): void {
    this.editBaseUrl = this.apiBaseUrl;
    this.editToken = this.apiToken;
    this.load(false);
    this.refreshTimer = setInterval(() => {
      if (this.autoRefresh && !this.inFlight) {
        this.load(true);
      }
    }, 5000);
    this.replayTimer = setInterval(() => {
      if (!this.replayActive || !this.replayPlaying) {
        return;
      }
      const length: number = this.timeline().length;
      if (length === 0) {
        return;
      }
      if (this.replayIndex >= length - 1) {
        this.replayPlaying = false;
      } else {
        this.replayIndex += 1;
      }
    }, 220);
  }

  aboutToDisappear(): void {
    if (this.refreshTimer >= 0) {
      clearInterval(this.refreshTimer);
      this.refreshTimer = -1;
    }
    if (this.replayTimer >= 0) {
      clearInterval(this.replayTimer);
      this.replayTimer = -1;
    }
    this.stopDemoLoop();
    if (this.framePixel !== undefined) {
      this.framePixel.release();
      this.framePixel = undefined;
    }
  }

  @Builder
  buildTabBar(tabIndex: number, icon: Resource, label: string) {
    Column({ space: 3 }) {
      SymbolGlyph(icon)
        .fontSize(22)
        .fontColor([this.activeTab === tabIndex ? COLORS.cyan : Color.White])
      Text(label)
        .fontSize(10)
        .fontWeight(FontWeight.Medium)
        .fontColor(this.activeTab === tabIndex ? COLORS.cyan : Color.White)
    }
    .width('100%')
    .height('100%')
    .justifyContent(FlexAlign.Center)
  }

  private titleBarContent(): TitleBarContentOptions {
    const items: HdsNavigationMenuItemOptions[] = [
      {
        content: {
          label: '刷新',
          icon: $r('sys.symbol.arrow_clockwise'),
          type: IconStyleMode.NORMAL,
          isEnabled: true,
          componentId: 'refresh',
          action: () => this.refresh()
        }
      },
      {
        content: {
          label: '设置',
          icon: $r('sys.symbol.gearshape'),
          type: IconStyleMode.NORMAL,
          isEnabled: true,
          componentId: 'settings',
          action: () => this.openSettings()
        }
      }
    ];
    const menu: HdsNavigationMenuContentOptions = { value: items };
    return {
      title: {
        mainTitle: '成长守护',
        subTitle: `${this.selectedDeviceLabel()} · 家长端`
      },
      menu: menu
    };
  }

  build() {
    Stack() {
      Column()
        .width('100%')
        .height('100%')
        .linearGradient({
          angle: 135,
          colors: [['#05070C', 0], ['#0A1220', 0.48], ['#130B22', 1]]
        })

      Circle({ width: 300, height: 300 })
        .fill('#2222D3EE')
        .blur(70)
        .position({ x: -90, y: -70 })
        .opacity(0.55)
      Circle({ width: 260, height: 260 })
        .fill('#22A78BFA')
        .blur(70)
        .position({ x: 230, y: 150 })
        .opacity(0.38)
      Circle({ width: 280, height: 280 })
        .fill('#1834D399')
        .blur(80)
        .position({ x: 40, y: 620 })
        .opacity(0.3)

      HdsNavigation(this.pathStack) {
        HdsTabs({ controller: this.tabsController, index: this.activeTab }) {
          TabContent() {
            this.buildOverview()
          }
          .tabBar(this.buildTabBar(AppTab.OVERVIEW, $r('sys.symbol.square_grid_2x2'), '总览'))

          TabContent() {
            this.buildReport()
          }
          .tabBar(this.buildTabBar(AppTab.REPORT, $r('sys.symbol.doc_text'), 'AI 报告'))

          TabContent() {
            this.buildMessages()
          }
          .tabBar(this.buildTabBar(AppTab.MESSAGES, $r('sys.symbol.bookmark_notepad'), '留言'))

          TabContent() {
            this.buildReplay()
          }
          .tabBar(this.buildTabBar(AppTab.REPLAY, $r('sys.symbol.clock'), '时间回放'))

          TabContent() {
            this.buildDemo()
          }
          .tabBar(this.buildTabBar(AppTab.DEMO, $r('sys.symbol.wand_and_stars_fill'), '演示'))
        }
        .vertical(false)
        .barPosition(BarPosition.End)
        .barMode(BarMode.Fixed)
        .scrollable(false)
        .animationDuration(240)
        .barHeight(56)
        .barOverlap(true)
        .expandSafeArea([SafeAreaType.SYSTEM], [SafeAreaEdge.TOP, SafeAreaEdge.BOTTOM])
        .height('100%')
        .barFloatingStyle({
          barSideMargin: 18,
          barBottomMargin: 22,
          thermoCtrl: true,
          adaptToHandedness: true,
          systemMaterialEffect: this.materialParams
        })
        .onChange((index: number) => {
          this.activeTab = index;
        })
      }
      .mode(NavigationMode.Stack)
      .titleBar({
        content: this.titleBarContent(),
        style: {
          scrollEffectOpts: {
            enableScrollEffect: false,
            scrollEffectType: ScrollEffectType.GRADIENT_BLUR
          },
          systemMaterialEffect: this.materialParams
        },
        avoidLayoutSafeArea: false,
        enableComponentSafeArea: false
      })
      .hideBackButton(true)
      .titleMode(HdsNavigationTitleMode.MINI)
      .backgroundColor(Color.Transparent)
      .ignoreLayoutSafeArea([LayoutSafeAreaType.SYSTEM], [LayoutSafeAreaEdge.TOP, LayoutSafeAreaEdge.BOTTOM])
      .width('100%')
      .height('100%')
    }
    .width('100%')
    .height('100%')
    .bindSheet($$this.showSettings, this.buildSettingsSheet(), {
      height: 440,
      dragBar: true,
      showClose: false,
      backgroundColor: SHEET_MATERIAL === undefined ? COLORS.bg : Color.Transparent,
      systemMaterial: SHEET_MATERIAL
    })
  }

  @Builder
  buildRefreshIndicator() {
    Row() {
      LoadingProgress()
        .width(24)
        .height(24)
        .color(COLORS.cyan)
    }
    .width('100%')
    .height(56)
    .justifyContent(FlexAlign.Center)
  }

  @Builder
  buildOverview() {
    Refresh({ refreshing: $$this.refreshing, builder: (): void => this.buildRefreshIndicator() }) {
      Scroll() {
        Column({ space: 14 }) {
          Row().height(TITLE_BAR_PLACEHOLDER)
          if (this.loading && this.devices.length === 0) {
            this.buildLoadingCard()
          } else if (this.devices.length === 0) {
            this.buildEmptyCard()
          } else {
            this.buildChildStrip()
            this.buildHeroCard()
            this.buildFrameCard()
            this.buildScoreCard()
            this.buildMetricGrid()
            this.buildLightCard()
            this.buildMixCard()
            this.buildEventCard()
          }
        }
        .width('100%')
        .padding({ left: 14, right: 14, bottom: TAB_BOTTOM_PADDING })
        .alignItems(HorizontalAlign.Start)
      }
      .scrollBar(BarState.Off)
      .edgeEffect(EdgeEffect.Spring)
      .width('100%')
      .height('100%')
    }
    .onRefreshing(() => {
      this.refresh();
    })
    .width('100%')
    .height('100%')
  }

  @Builder
  buildReport() {
    Scroll() {
      Column({ space: 14 }) {
        Row().height(TITLE_BAR_PLACEHOLDER)
        if (this.devices.length === 0) {
          this.buildEmptyCard()
        } else {
          this.buildAIReportCard()
          this.buildHeatmapCard()
        }
      }
      .width('100%')
      .padding({ left: 14, right: 14, bottom: TAB_BOTTOM_PADDING })
      .alignItems(HorizontalAlign.Start)
    }
    .scrollBar(BarState.Off)
    .edgeEffect(EdgeEffect.Spring)
    .width('100%')
    .height('100%')
  }

  @Builder
  buildMessages() {
    Scroll() {
      Column({ space: 14 }) {
        Row().height(TITLE_BAR_PLACEHOLDER)
        if (this.devices.length === 0) {
          this.buildEmptyCard()
        } else {
          GlassCard({
            kicker: 'FAMILY / MESSAGES',
            title: '家庭留言',
            subtitle: '家长和孩子的话会出现在 ESP 屏幕上'
          }) {
            Column({ space: 10 }) {
              if (this.messages.length === 0) {
                Column({ space: 6 }) {
                  SymbolGlyph($r('sys.symbol.bookmark_notepad'))
                    .fontSize(28)
                    .fontColor([COLORS.dim])
                  Text('还没有家庭留言')
                    .fontSize(14)
                    .fontColor(COLORS.text)
                  Text('写一句鼓励的话，ESP 会在几秒内显示。')
                    .fontSize(10)
                    .fontColor(COLORS.dim)
                }
                .width('100%')
                .height(140)
                .justifyContent(FlexAlign.Center)
              } else {
                ForEach(this.messages, (message: Message) => {
                  Row({ space: 10 }) {
                    Stack({ alignContent: Alignment.Center }) {
                      Circle({ width: 34, height: 34 })
                        .fill(message.author === 'parent' ? '#1422D3EE' : '#14FBBF24')
                        .stroke(message.author === 'parent' ? '#4022D3EE' : '#40FBBF24')
                        .strokeWidth(1)
                      Text(message.author === 'parent' ? '妈' : '宇')
                        .fontSize(12)
                        .fontColor(message.author === 'parent' ? '#A5F3FC' : '#FDE68A')
                    }
                    Column({ space: 3 }) {
                      Row({ space: 6 }) {
                        Text(message.author_name)
                          .fontSize(12)
                          .fontWeight(FontWeight.Medium)
                          .fontColor(COLORS.text)
                        Text(message.author === 'parent' ? '家长' : '孩子')
                          .fontSize(7)
                          .fontColor(COLORS.dim)
                          .padding({ left: 4, right: 4, top: 1, bottom: 1 })
                          .backgroundColor('#0F94A3B8')
                          .borderRadius(6)
                        Blank()
                        Text(formatRelative(message.created_at))
                          .fontSize(8)
                          .fontColor(COLORS.dim)
                      }
                      .width('100%')
                      Text(message.content)
                        .fontSize(11)
                        .fontColor(COLORS.muted)
                        .lineHeight(17)
                      Row({ space: 4 }) {
                        SymbolGlyph(message.delivered_at !== undefined && message.delivered_at !== null
                          ? $r('sys.symbol.checkmark')
                          : $r('sys.symbol.clock'))
                          .fontSize(11)
                          .fontColor([message.delivered_at !== undefined && message.delivered_at !== null
                            ? COLORS.emerald : COLORS.dim])
                        Text(message.delivered_at !== undefined && message.delivered_at !== null
                          ? '已送达 ESP 屏幕' : '等待 ESP 拉取')
                          .fontSize(8)
                          .fontColor(COLORS.dim)
                      }
                    }
                    .layoutWeight(1)
                    .alignItems(HorizontalAlign.Start)
                  }
                  .width('100%')
                  .padding(10)
                  .borderRadius(12)
                  .backgroundColor(message.author === 'parent' ? '#0F22D3EE' : '#0FFBBF24')
                  .border({ width: 1, color: message.author === 'parent' ? '#1F22D3EE' : '#1FFBBF24' })
                }, (message: Message) => `${message.id}`)
              }

              Row({ space: 6 }) {
                Button('家长 · 妈妈')
                  .fontSize(10)
                  .fontColor(this.messageRole === 'parent' ? COLORS.cyan : COLORS.muted)
                  .backgroundColor(this.messageRole === 'parent' ? '#1422D3EE' : '#0F94A3B8')
                  .height(32)
                  .borderRadius(16)
                  .layoutWeight(1)
                  .onClick(() => {
                    this.messageRole = 'parent';
                  })
                Button('孩子 · 小宇')
                  .fontSize(10)
                  .fontColor(this.messageRole === 'child' ? COLORS.amber : COLORS.muted)
                  .backgroundColor(this.messageRole === 'child' ? '#14FBBF24' : '#0F94A3B8')
                  .height(32)
                  .borderRadius(16)
                  .layoutWeight(1)
                  .onClick(() => {
                    this.messageRole = 'child';
                  })
              }
              .width('100%')

              TextArea({ text: this.messageDraft, placeholder: '写一句留言…' })
                .fontSize(12)
                .fontColor(COLORS.text)
                .placeholderColor(COLORS.dim)
                .backgroundColor('#0F94A3B8')
                .borderRadius(10)
                .height(92)
                .padding(10)
                .onChange((value: string) => {
                  this.messageDraft = value;
                })

              Row({ space: 8 }) {
                Text(`${this.messageDraft.length}/500`)
                  .fontSize(8)
                  .fontColor(COLORS.dim)
                Blank()
                Button(this.messageSending ? '发送中…' : '发送')
                  .fontSize(10)
                  .fontColor(Color.Black)
                  .backgroundColor(COLORS.cyan)
                  .height(32)
                  .borderRadius(16)
                  .enabled(!this.messageSending && this.messageDraft.trim().length > 0)
                  .onClick(() => {
                    void this.sendMessage();
                  })
              }
              .width('100%')

              if (this.messageError.length > 0) {
                Text(this.messageError)
                  .fontSize(9)
                  .fontColor('#FECDD3')
                  .padding(8)
                  .width('100%')
                  .backgroundColor('#14FB7185')
                  .borderRadius(8)
              }

              Text('ESP 每 10 秒左右拉取一次；留言会显示在表情屏底部。')
                .fontSize(8)
                .fontColor(COLORS.dim)
                .lineHeight(13)
            }
            .width('100%')
          }
        }
      }
      .width('100%')
      .padding({ left: 14, right: 14, bottom: TAB_BOTTOM_PADDING })
      .alignItems(HorizontalAlign.Start)
    }
    .scrollBar(BarState.Off)
    .edgeEffect(EdgeEffect.Spring)
    .width('100%')
    .height('100%')
  }

  @Builder
  buildReplay() {
    Scroll() {
      Column({ space: 14 }) {
        Row().height(TITLE_BAR_PLACEHOLDER)
        if (this.devices.length === 0) {
          this.buildEmptyCard()
        } else {
          this.buildReplayCard()
          this.buildReplayListCard()
        }
      }
      .width('100%')
      .padding({ left: 14, right: 14, bottom: TAB_BOTTOM_PADDING })
      .alignItems(HorizontalAlign.Start)
    }
    .scrollBar(BarState.Off)
    .edgeEffect(EdgeEffect.Spring)
    .width('100%')
    .height('100%')
  }

  @Builder
  buildDemo() {
    Scroll() {
      Column({ space: 14 }) {
        Row().height(TITLE_BAR_PLACEHOLDER)
        if (this.devices.length === 0) {
          this.buildEmptyCard()
        } else {
          this.buildDemoCard()
          this.buildConnectionCard()
          this.buildNativeCard()
        }
      }
      .width('100%')
      .padding({ left: 14, right: 14, bottom: TAB_BOTTOM_PADDING })
      .alignItems(HorizontalAlign.Start)
    }
    .scrollBar(BarState.Off)
    .edgeEffect(EdgeEffect.Spring)
    .width('100%')
    .height('100%')
  }

  @Builder
  buildLoadingCard() {
    GlassCard({ kicker: 'CONNECTING', title: '正在连接成长守护数据', subtitle: this.apiBaseUrl }) {
      Row({ space: 10 }) {
        LoadingProgress()
          .width(26)
          .height(26)
          .color(COLORS.cyan)
        Text('正在读取设备、标注画面与 AI 报告…')
          .fontSize(11)
          .fontColor(COLORS.muted)
      }
      .width('100%')
      .padding({ top: 8, bottom: 8 })
    }
  }

  @Builder
  buildEmptyCard() {
    GlassCard({ kicker: 'NO DATA', title: '还没有孩子的坐姿数据', subtitle: '真机与 Mac 需要在同一网络，并且 Go 服务监听 0.0.0.0:11294' }) {
      Column({ space: 10 }) {
        if (this.errorMessage.length > 0) {
          Text(this.errorMessage)
            .fontSize(10)
            .fontColor('#FECDD3')
            .padding(9)
            .width('100%')
            .backgroundColor('#14FB7185')
            .borderRadius(9)
        }
        Text(`当前地址：${this.apiBaseUrl}`)
          .fontSize(9)
          .fontColor(COLORS.dim)
        Row({ space: 8 }) {
          Button('打开设置')
            .fontSize(11)
            .fontColor(Color.Black)
            .backgroundColor(COLORS.cyan)
            .height(32)
            .borderRadius(16)
            .onClick(() => this.openSettings())
          Button('立即重试')
            .fontSize(11)
            .fontColor(COLORS.cyan)
            .backgroundColor('#1422D3EE')
            .height(32)
            .borderRadius(16)
            .onClick(() => this.refresh())
        }
      }
      .width('100%')
      .alignItems(HorizontalAlign.Start)
    }
  }

  @Builder
  buildChildStrip() {
    Row({ space: 10 }) {
      Stack({ alignContent: Alignment.Center }) {
        Circle({ width: 38, height: 38 })
          .fill('#1AFBBF24')
          .stroke('#4DFBBF24')
          .strokeWidth(1)
        Text('宇')
          .fontSize(14)
          .fontWeight(FontWeight.Medium)
          .fontColor('#FEF3C7')
      }
      Column({ space: 2 }) {
        Text('小宇')
          .fontSize(13)
          .fontWeight(FontWeight.Medium)
          .fontColor(COLORS.text)
        Text('9 岁 · 三年级 · 演示档案')
          .fontSize(9)
          .fontColor(COLORS.dim)
      }
      .alignItems(HorizontalAlign.Start)
      Blank()
      Column({ space: 2 }) {
        Text('LIVE')
          .fontSize(8)
          .fontColor(COLORS.emerald)
          .letterSpacing(0.8)
        Text(this.lastUpdated.length > 0 ? formatRelative(this.lastUpdated) : '等待同步')
          .fontSize(9)
          .fontColor(COLORS.muted)
      }
      .alignItems(HorizontalAlign.End)
    }
    .width('100%')
    .padding({ left: 11, right: 11, top: 8, bottom: 8 })
    .borderRadius(14)
    .backgroundColor('#14FBBF24')
    .border({ width: 1, color: '#1FFBBF24' })

    Row({ space: 8 }) {
      Select(this.deviceOptions())
        .selected(this.selectedDeviceIndex)
        .value(this.selectedDeviceLabel())
        .font({ size: 12 })
        .fontColor(COLORS.text)
        .selectedOptionBgColor(COLORS.cyan)
        .selectedOptionFontColor(Color.Black)
        .optionBgColor('#0B111C')
        .optionFontColor(COLORS.text)
        .menuAlign(MenuAlignType.END)
        .onSelect((index: number) => {
          this.selectDevice(index);
        })
        .layoutWeight(1)
      Row({ space: 4 }) {
        ForEach(RANGES, (range: string) => {
          Text(range)
            .fontSize(9)
            .fontColor(this.activeRange === range ? COLORS.cyan : COLORS.dim)
            .padding({ left: 7, right: 7, top: 5, bottom: 5 })
            .borderRadius(8)
            .backgroundColor(this.activeRange === range ? '#1622D3EE' : '#0F94A3B8')
            .onClick(() => this.changeRange(range))
        }, (range: string) => range)
      }
    }
    .width('100%')
    .margin({ top: -4 })
  }

  @Builder
  buildHeroCard() {
    GlassCard({
      kicker: 'POSTURE / CHILD',
      title: '孩子当前坐姿',
      subtitle: '实时姿态识别 · 家长端守护'
    }) {
      Row({ space: 12 }) {
        Face({ status: this.displayStatus(), person: this.displayPerson() })
        Column({ space: 6 }) {
          Row({ space: 6 }) {
            StatusPill({ status: this.displayStatus() })
            if (this.replayActive) {
              Text('回放')
                .fontSize(8)
                .fontColor(COLORS.violet)
                .padding({ left: 5, right: 5, top: 2, bottom: 2 })
                .border({ width: 1, color: '#40A78BFA' })
                .borderRadius(8)
            }
          }
          Text(this.displaySummary())
            .fontSize(19)
            .fontWeight(FontWeight.Medium)
            .fontColor(COLORS.text)
            .maxLines(2)
            .lineHeight(25)
          Text(this.displayPerson()
            ? '已检测到孩子，正在分析坐姿与身体偏移。'
            : '孩子暂时离开书桌，设备保持守护状态。')
            .fontSize(10)
            .fontColor(COLORS.muted)
            .lineHeight(15)
          Flex({ wrap: FlexWrap.Wrap }) {
            TinyChip({
              label: this.displayAlert() ? '告警中' : '无告警',
              color: this.displayAlert() ? COLORS.rose : COLORS.emerald
            })
            if (this.displaySedentary()) {
              TinyChip({ label: '久坐提醒', color: COLORS.amber })
            }
            TinyChip({ label: `issue ${this.displayIssueCount()}`, color: COLORS.dim })
            TinyChip({ label: `seq ${this.displaySequence()}`, color: COLORS.dim })
            TinyChip({ label: formatRelative(this.displayLastSeen()), color: COLORS.dim })
          }
        }
        .layoutWeight(1)
        .alignItems(HorizontalAlign.Start)
      }
      .width('100%')
      .alignItems(VerticalAlign.Top)

      TimelineStrip({ timeline: this.recentTimeline(), activeIndex: -1 })

      Row({ space: 14 }) {
        Text(`uptime ${this.formatUptime(this.latestNumber('DeviceStatus', 'uptime', 0))}`)
          .fontSize(8)
          .fontColor(COLORS.dim)
        Text(`fw ${this.latestText('DeviceStatus', 'firmware', '—')}`)
          .fontSize(8)
          .fontColor(COLORS.dim)
        Text(`report ${this.latestNumber('DeviceStatus', 'reportInterval', 0)}s`)
          .fontSize(8)
          .fontColor(COLORS.dim)
      }
      .width('100%')
      .padding({ top: 8 })
      .border({ width: { top: 1 }, color: '#1A94A3B8' })
    }
  }

  @Builder
  buildFrameCard() {
    GlassCard({
      kicker: 'CAMERA / POSTURE VISION',
      title: '孩子的实时坐姿画面',
      subtitle: '本地 MediaPipe 关键点识别 · 标注图自动上传'
    }) {
      Row({ space: 6 }) {
        this.buildTag('AI 姿态标注', COLORS.cyan)
        this.buildTag('SNAPSHOT', COLORS.emerald)
        Blank()
        Text(`${this.frames.length} FRAMES`)
          .fontSize(8)
          .fontColor(COLORS.dim)
          .letterSpacing(0.6)
      }
      .width('100%')

      Stack({ alignContent: Alignment.Center }) {
        if (this.framePixel !== undefined) {
          Image(this.framePixel as image.PixelMap)
            .width('100%')
            .aspectRatio(4 / 3)
            .objectFit(ImageFit.Contain)
            .borderRadius(12)
            .backgroundColor('#020409')
            .interpolation(ImageInterpolation.High)
        } else {
          Column({ space: 8 }) {
            SymbolGlyph($r('sys.symbol.camera'))
              .fontSize(28)
              .fontColor([COLORS.dim])
            Text(this.frameLoading ? '正在加载标注画面…' : '还没有标注画面')
              .fontSize(12)
              .fontColor(COLORS.text)
            Text('本地识别服务生成 _annotated.jpg 后会自动上传')
              .fontSize(9)
              .fontColor(COLORS.dim)
              .textAlign(TextAlign.Center)
          }
          .width('100%')
          .height(190)
          .justifyContent(FlexAlign.Center)
          .backgroundColor('#050A12')
          .borderRadius(12)
          .border({ width: 1, color: '#1A22D3EE' })
        }

        if (this.framePixel !== undefined && this.selectedFrame() !== undefined) {
          Row({ space: 8 }) {
            Text(statusMeta(this.frameStatus(this.selectedFrame() as PostureFrame)).zh)
              .fontSize(9)
              .fontColor(statusMeta(this.frameStatus(this.selectedFrame() as PostureFrame)).color)
            Text(this.selectedFrame()?.summary ?? '')
              .fontSize(10)
              .fontColor(COLORS.text)
              .maxLines(1)
              .textOverflow({ overflow: TextOverflow.Ellipsis })
              .layoutWeight(1)
            Text(this.selectedFrame() === undefined ? '—' :
              formatClock((this.selectedFrame() as PostureFrame).captured_at))
              .fontSize(8)
              .fontColor(COLORS.muted)
          }
          .width('100%')
          .padding({ left: 10, right: 10, top: 6, bottom: 6 })
          .backgroundColor('#B3050A12')
          .borderRadius({ bottomLeft: 12, bottomRight: 12 })
          .align(Alignment.Bottom)
        }
      }
      .width('100%')
      .clip(true)

      if (this.frameError.length > 0) {
        Text(this.frameError)
          .fontSize(9)
          .fontColor('#FECDD3')
      }

      if (this.frames.length > 0) {
        Scroll() {
          Row({ space: 6 }) {
            ForEach(this.frames, (frame: PostureFrame) => {
              Column({ space: 2 }) {
                Row({ space: 4 }) {
                  Circle({ width: 6, height: 6 })
                    .fill(statusMeta(this.frameStatus(frame)).color)
                  Text(formatClock(frame.captured_at))
                    .fontSize(10)
                    .fontColor(COLORS.text)
                }
                Text(statusMeta(this.frameStatus(frame)).label)
                  .fontSize(7)
                  .fontColor(COLORS.dim)
              }
              .padding({ left: 8, right: 8, top: 6, bottom: 6 })
              .borderRadius(9)
              .border({
                width: 1,
                color: this.selectedFrameId === frame.id ? '#4022D3EE' : '#1494A3B8'
              })
              .backgroundColor(this.selectedFrameId === frame.id ? '#1422D3EE' : '#0F94A3B8')
              .onClick(() => this.selectFrame(frame.id))
            }, (frame: PostureFrame) => `${frame.id}`)
          }
        }
        .scrollable(ScrollDirection.Horizontal)
        .scrollBar(BarState.Off)
        .width('100%')
      }
    }
  }

  @Builder
  buildTag(label: string, color: string) {
    Text(label)
      .fontSize(8)
      .fontColor(color)
      .padding({ left: 6, right: 6, top: 3, bottom: 3 })
      .border({ width: 1, color: '#2694A3B8' })
      .borderRadius(8)
      .backgroundColor('#0F050A12')
  }

  @Builder
  buildScoreCard() {
    GlassCard({
      kicker: 'GROWTH / SCORE',
      title: '成长守护评分',
      subtitle: '最近 24 小时 · 坐姿 + 环境综合评估'
    }) {
      Row({ space: 14 }) {
        Stack({ alignContent: Alignment.Center }) {
          Progress({ value: this.score(), total: 100, type: ProgressType.Ring })
            .width(104)
            .height(104)
            .color(scoreColor(this.score()))
            .backgroundColor('#1A94A3B8')
            .style({ strokeWidth: 9 })
          Column({ space: 1 }) {
            Text(`${this.score()}`)
              .fontSize(30)
              .fontWeight(FontWeight.Medium)
              .fontColor(scoreColor(this.score()))
            Text('成长评分')
              .fontSize(8)
              .fontColor(COLORS.dim)
              .letterSpacing(0.6)
          }
        }
        .width(108)
        .height(108)

        Column({ space: 5 }) {
          Row({ space: 5 }) {
            Text(this.scoreLevelLabel())
              .fontSize(9)
              .fontColor(scoreColor(this.score()))
              .padding({ left: 6, right: 6, top: 2, bottom: 2 })
              .border({ width: 1, color: '#2694A3B8' })
              .borderRadius(8)
            if (this.aiLoading) {
              LoadingProgress()
                .width(14)
                .height(14)
                .color(COLORS.violet)
            }
          }
          Text(this.aiReport !== null ? (this.aiReport as AIReport).headline : '今天也在认真守护成长')
            .fontSize(15)
            .fontWeight(FontWeight.Medium)
            .fontColor(COLORS.text)
            .maxLines(2)
          Text(this.aiReport !== null ? (this.aiReport as AIReport).summary :
            '评分综合坐姿良好率、提醒频率、光照舒适度与在座时长。')
            .fontSize(9)
            .fontColor(COLORS.muted)
            .lineHeight(14)
            .maxLines(3)
            .textOverflow({ overflow: TextOverflow.Ellipsis })
          Button(this.aiReport === null ? '生成 AI 健康报告' : '重新生成 AI 报告')
            .fontSize(10)
            .fontColor(Color.Black)
            .backgroundColor(COLORS.cyan)
            .height(30)
            .borderRadius(15)
            .enabled(!this.aiLoading)
            .onClick(() => this.generateReport())
        }
        .layoutWeight(1)
        .alignItems(HorizontalAlign.Start)
      }
      .width('100%')
      .alignItems(VerticalAlign.Top)

      Row({ space: 8 }) {
        MiniStat({
          label: '今日良好率',
          value: this.summaryPercent(),
          hint: this.summaryGoodHint(),
          accent: COLORS.emerald
        })
        MiniStat({
          label: '今日提醒',
          value: this.summaryAlerts(),
          hint: '累计告警分钟',
          accent: COLORS.rose
        })
      }
      .width('100%')
      Row({ space: 8 }) {
        MiniStat({
          label: '最长告警',
          value: this.summaryStreak(),
          hint: '连续不良坐姿',
          accent: COLORS.amber
        })
        MiniStat({
          label: '平均光照',
          value: this.summaryLight(),
          hint: '舒适度',
          accent: COLORS.cyan
        })
      }
      .width('100%')

      if (this.aiError.length > 0) {
        Text(this.aiError)
          .fontSize(9)
          .fontColor('#FECDD3')
      }
    }
  }

  @Builder
  buildMetricGrid() {
    Row({ space: 8 }) {
      Column({ space: 8 }) {
        MetricTile({
          icon: $r('sys.symbol.brightness'),
          label: '桌面光照',
          value: formatNumber(this.displayLight(), 0),
          unit: '%',
          hint: this.displayLight() >= 60 ? '光照充足' : this.displayLight() >= 30 ? '光照适中' : '光照偏暗',
          accent: COLORS.amber,
          live: !this.replayActive
        })
        MetricTile({
          icon: $r('sys.symbol.battery_bolt_fill'),
          label: '桌面温度',
          value: formatNumber(this.latestNumber('Environment', 'temperature', 0), 1),
          unit: '°C',
          hint: '传感器离线 · 固定上报 0',
          accent: COLORS.rose
        })
        MetricTile({
          icon: $r('sys.symbol.line_3_horizontal'),
          label: '设备信号',
          value: formatNumber(this.latestNumber('DeviceStatus', 'rssi', 0), 0),
          unit: 'dBm',
          hint: 'RSSI',
          accent: COLORS.emerald,
          live: !this.replayActive
        })
      }
      .layoutWeight(1)
      Column({ space: 8 }) {
        MetricTile({
          icon: $r('sys.symbol.bell'),
          label: '环境声音',
          value: formatNumber(this.displaySound(), 0),
          unit: 'LEVEL',
          hint: this.displaySound() === 1 ? '检测到声音' : '环境安静',
          accent: COLORS.violet
        })
        MetricTile({
          icon: $r('sys.symbol.drop_fill'),
          label: '桌面湿度',
          value: formatNumber(this.latestNumber('Environment', 'humidity', 0), 0),
          unit: '%',
          hint: '传感器离线 · 固定上报 0',
          accent: COLORS.cyan
        })
        MetricTile({
          icon: $r('sys.symbol.hash'),
          label: '设备内存',
          value: formatNumber(this.latestNumber('DeviceStatus', 'freeHeap', 0) / 1024, 0),
          unit: 'KB',
          hint: `uptime ${this.formatUptime(this.latestNumber('DeviceStatus', 'uptime', 0))}`,
          accent: COLORS.slate
        })
      }
      .layoutWeight(1)
    }
    .width('100%')
    .alignItems(VerticalAlign.Top)
  }

  @Builder
  buildLightCard() {
    GlassCard({
      kicker: 'ENVIRONMENT / LIGHT',
      title: '桌面光照',
      subtitle: `最近 ${this.activeRange} · ${this.activeRange === '1H' || this.activeRange === '6H' ? '每分钟聚合' : '每小时聚合'}`
    }) {
      Row({ space: 5 }) {
        ForEach(RANGES, (range: string) => {
          Text(range)
            .fontSize(8)
            .fontColor(this.activeRange === range ? COLORS.cyan : COLORS.dim)
            .padding({ left: 7, right: 7, top: 4, bottom: 4 })
            .borderRadius(7)
            .backgroundColor(this.activeRange === range ? '#1622D3EE' : '#0F94A3B8')
            .onClick(() => this.changeRange(range))
        }, (range: string) => `light-${range}`)
        Blank()
        Text('LIGHT TRACE')
          .fontSize(7)
          .fontColor(COLORS.dim)
          .letterSpacing(0.8)
      }
      .width('100%')

      if (this.lightPoints.length < 2) {
        Text('等待光照时序数据')
          .fontSize(10)
          .fontColor(COLORS.dim)
          .width('100%')
          .height(100)
          .textAlign(TextAlign.Center)
      } else {
        SparkLine({ points: this.lightPoints, accent: COLORS.cyan, chartHeight: 120 })
      }

      Row({ space: 12 }) {
        ChartStat({ label: 'NOW', value: `${formatNumber(this.displayLight(), 0)}%` })
        ChartStat({ label: 'AVG', value: this.lightAverage() })
        ChartStat({ label: 'PEAK', value: this.lightPeak() })
        Blank()
        Text(`${this.lightPoints.length} 个聚合点`)
          .fontSize(8)
          .fontColor(COLORS.dim)
      }
      .width('100%')
    }
  }

  @Builder
  buildMixCard() {
    GlassCard({
      kicker: 'POSTURE / DISTRIBUTION',
      title: '坐姿分布',
      subtitle: '最近姿态样本占比 · 覆盖今日守护'
    }) {
      if (this.insights !== null && (this.insights as Insights).summary.posture_samples > 0) {
        Row() {
          ForEach(this.mixItems(), (item: MixItem) => {
            if (item.count > 0) {
              Column()
                .layoutWeight(item.count)
                .height(12)
                .backgroundColor(item.color)
            }
          }, (item: MixItem) => item.key)
        }
        .width('100%')
        .borderRadius(6)
        .clip(true)

        Column({ space: 6 }) {
          ForEach(this.mixItems(), (item: MixItem) => {
            Row({ space: 7 }) {
              Circle({ width: 7, height: 7 }).fill(item.color)
              Text(item.label)
                .fontSize(9)
                .fontColor(COLORS.muted)
              Blank()
              Text(`${item.count}`)
                .fontSize(9)
                .fontColor(COLORS.text)
              Text(this.mixPercent(item))
                .fontSize(9)
                .fontColor(COLORS.dim)
                .width(36)
                .textAlign(TextAlign.End)
            }
            .width('100%')
          }, (item: MixItem) => `mix-${item.key}`)
        }
        .width('100%')
      } else {
        Text('等待姿态样本')
          .fontSize(10)
          .fontColor(COLORS.dim)
      }
    }
  }

  @Builder
  buildEventCard() {
    GlassCard({
      kicker: 'IOTDA / EVENT STREAM',
      title: '守护事件流',
      subtitle: '孩子的设备最近上报记录'
    }) {
      if (this.events.length === 0) {
        Text('等待 IoTDA 推送事件')
          .fontSize(10)
          .fontColor(COLORS.dim)
      } else {
        Column({ space: 0 }) {
          ForEach(this.events.slice(0, 6), (event: EventRow) => {
            Row({ space: 9 }) {
              Circle({ width: 7, height: 7 })
                .fill(this.eventColor(event))
                .margin({ top: 5 })
              Column({ space: 2 }) {
                Text(this.eventTitle(event))
                  .fontSize(11)
                  .fontColor(COLORS.text)
                  .maxLines(1)
                  .textOverflow({ overflow: TextOverflow.Ellipsis })
                Text(`${formatClock(event.event_time ?? event.received_at)} · ${formatRelative(event.event_time ?? event.received_at)}`)
                  .fontSize(8)
                  .fontColor(COLORS.dim)
              }
              .layoutWeight(1)
              .alignItems(HorizontalAlign.Start)
            }
            .width('100%')
            .padding({ top: 8, bottom: 8 })
            .border({ width: { bottom: 1 }, color: '#1494A3B8' })
          }, (event: EventRow) => `${event.id}`)
        }
        .width('100%')
      }
    }
  }

  @Builder
  buildAIReportCard() {
    if (this.aiLoading && this.aiReport === null) {
      GlassCard({ kicker: 'AI / GROWTH ADVISOR', title: 'AI 健康报告', subtitle: 'GPT-4o 正在阅读孩子的 24 小时数据…' }) {
        Row({ space: 10 }) {
          LoadingProgress().width(26).height(26).color(COLORS.violet)
          Text('分析良好率、告警时段、光照舒适度与连续坐姿…')
            .fontSize(10)
            .fontColor(COLORS.muted)
        }
        .width('100%')
        .padding({ top: 8, bottom: 8 })
      }
    } else if (this.aiReport !== null) {
      GlassCard({ kicker: 'AI / GROWTH ADVISOR', title: 'AI 健康报告', subtitle: '给家长的可执行建议 · 不是医疗诊断' }) {
        AIReportView({
          report: this.aiReport as AIReport,
          onRegenerate: () => {
            this.generateReport();
          }
        })
      }
    } else {
      GlassCard({ kicker: 'AI / GROWTH ADVISOR', title: 'AI 健康报告', subtitle: '基于最近 24 小时坐姿与环境数据' }) {
        Column({ space: 10 }) {
          Row({ space: 10 }) {
            SymbolGlyph($r('sys.symbol.wand_and_stars_fill'))
              .fontSize(26)
              .fontColor([COLORS.violet])
            Column({ space: 3 }) {
              Text('生成一份给家长的坐姿健康报告')
                .fontSize(14)
                .fontColor(COLORS.text)
              Text('AI 会结合良好率、提醒次数、风险时段与光照，给出温和、可执行的建议。')
                .fontSize(9)
                .fontColor(COLORS.muted)
                .lineHeight(14)
            }
            .layoutWeight(1)
            .alignItems(HorizontalAlign.Start)
          }
          if (this.aiError.length > 0) {
            Text(this.aiError)
              .fontSize(9)
              .fontColor('#FECDD3')
              .padding(8)
              .width('100%')
              .backgroundColor('#14FB7185')
              .borderRadius(8)
          }
          Button('生成 AI 报告')
            .fontSize(11)
            .fontColor(Color.Black)
            .backgroundColor(COLORS.cyan)
            .height(32)
            .borderRadius(16)
            .onClick(() => this.generateReport())
        }
        .width('100%')
        .alignItems(HorizontalAlign.Start)
      }
    }
  }

  @Builder
  buildHeatmapCard() {
    GlassCard({
      kicker: 'PATTERN / 7 × 24',
      title: '一周坐姿热力图',
      subtitle: '按星期与小时聚合 · 颜色越深问题越集中'
    }) {
      Row({ space: 5 }) {
        this.buildMetricChip('alert', '告警率', COLORS.rose)
        this.buildMetricChip('good', '良好率', COLORS.emerald)
        this.buildMetricChip('light', '光照', COLORS.amber)
        Blank()
        Text('7 天聚合')
          .fontSize(8)
          .fontColor(COLORS.dim)
      }
      .width('100%')

      if (this.heatmapCells().length === 0) {
        Column({ space: 6 }) {
          SymbolGlyph($r('sys.symbol.square_grid_2x2'))
            .fontSize(24)
            .fontColor([COLORS.dim])
          Text('还没有一周的数据')
            .fontSize(12)
            .fontColor(COLORS.text)
          Text('设备持续上报后，这里会显示 7×24 小时的坐姿规律。')
            .fontSize(9)
            .fontColor(COLORS.dim)
        }
        .width('100%')
        .height(150)
        .justifyContent(FlexAlign.Center)
      } else {
        Heatmap({ cells: this.heatmapCells(), metric: this.heatMetric })
      }

      Row({ space: 8 }) {
        SymbolGlyph($r('sys.symbol.exclamationmark_circle_fill'))
          .fontSize(14)
          .fontColor([COLORS.rose])
        Text(this.topRiskText())
          .fontSize(9)
          .fontColor(COLORS.muted)
          .layoutWeight(1)
      }
      .width('100%')
      .padding(9)
      .backgroundColor('#14FB7185')
      .borderRadius(9)
    }
  }

  @Builder
  buildMetricChip(key: string, label: string, color: string) {
    Text(label)
      .fontSize(8)
      .fontColor(this.heatMetric === key ? color : COLORS.dim)
      .padding({ left: 7, right: 7, top: 4, bottom: 4 })
      .borderRadius(7)
      .backgroundColor(this.heatMetric === key ? '#1494A3B8' : '#0A94A3B8')
      .onClick(() => {
        this.heatMetric = key;
      })
  }

  @Builder
  buildReplayCard() {
    GlassCard({
      kicker: 'REPLAY / 24H',
      title: '24 小时时间回放',
      subtitle: '拖动时间轴，回看孩子今天的坐姿变化'
    }) {
      if (this.timeline().length === 0) {
        Text('还没有可回放的数据')
          .fontSize(10)
          .fontColor(COLORS.dim)
      } else {
        Row({ space: 10 }) {
          Column({ space: 3 }) {
            Text(this.replayActive ? 'REPLAY TIME' : 'LATEST SAMPLES')
              .fontSize(8)
              .fontColor(COLORS.dim)
              .letterSpacing(0.8)
            Text(formatDateTime(this.replayCurrentTime()))
              .fontSize(18)
              .fontWeight(FontWeight.Medium)
              .fontColor(COLORS.text)
            Text(`第 ${this.replayDisplayIndex() + 1} / ${this.timeline().length} 个采样点`)
              .fontSize(9)
              .fontColor(COLORS.dim)
          }
          .layoutWeight(1)
          .alignItems(HorizontalAlign.Start)

          Column({ space: 2 }) {
            Circle({ width: 7, height: 7 })
              .fill(statusMeta(this.replayStatus()).color)
            Text(statusMeta(this.replayStatus()).label)
              .fontSize(9)
              .fontColor(statusMeta(this.replayStatus()).color)
            Text(statusMeta(this.replayStatus()).zh)
              .fontSize(8)
              .fontColor(COLORS.dim)
          }
          .alignItems(HorizontalAlign.End)
        }
        .width('100%')

        Slider({
          value: this.replayDisplayIndex(),
          min: 0,
          max: Math.max(0, this.timeline().length - 1),
          step: 1
        })
          .selectedColor(COLORS.cyan)
          .trackColor('#1A94A3B8')
          .blockColor(COLORS.cyan)
          .onChange((value: number) => {
            this.replayIndex = Math.round(value);
            this.replayActive = true;
            this.replayPlaying = false;
          })
          .width('100%')

        Row({ space: 7 }) {
          Text(`光照 ${this.replayActive ? this.replayLight() : this.displayLight()}%`)
            .fontSize(9)
            .fontColor(COLORS.amber)
            .padding({ left: 7, right: 7, top: 4, bottom: 4 })
            .backgroundColor('#0F94A3B8')
            .borderRadius(8)
          Text(this.replayPerson() ? '在座' : '离开')
            .fontSize(9)
            .fontColor(COLORS.muted)
            .padding({ left: 7, right: 7, top: 4, bottom: 4 })
            .backgroundColor('#0F94A3B8')
            .borderRadius(8)
          Text(this.replayAlert() ? '坐姿提醒' : '无提醒')
            .fontSize(9)
            .fontColor(this.replayAlert() ? COLORS.rose : COLORS.muted)
            .padding({ left: 7, right: 7, top: 4, bottom: 4 })
            .backgroundColor('#0F94A3B8')
            .borderRadius(8)
        }
        .width('100%')

        SparkLine({ points: this.replayLightPoints(), accent: COLORS.cyan, chartHeight: 78 })

        TimelineStrip({
          timeline: this.timeline(),
          activeIndex: this.replayIndex,
          onSelect: (index: number) => {
            this.replayIndex = index;
            this.replayActive = true;
            this.replayPlaying = false;
          }
        })

        Row({ space: 8 }) {
          if (!this.replayActive) {
            Button('开始回放')
              .fontSize(11)
              .fontColor(Color.Black)
              .backgroundColor(COLORS.cyan)
              .height(32)
              .borderRadius(16)
              .onClick(() => this.startReplay())
          } else {
            Button(this.replayPlaying ? '暂停' : '继续')
              .fontSize(11)
              .fontColor(Color.Black)
              .backgroundColor(COLORS.cyan)
              .height(32)
              .borderRadius(16)
              .onClick(() => this.toggleReplayPlaying())
            Button('回到实时')
              .fontSize(11)
              .fontColor(COLORS.cyan)
              .backgroundColor('#1422D3EE')
              .height(32)
              .borderRadius(16)
              .onClick(() => this.exitReplay())
          }
          Blank()
          Text('每 220ms 前进 5 分钟')
            .fontSize(8)
            .fontColor(COLORS.dim)
        }
        .width('100%')
      }
    }
  }

  @Builder
  buildReplayListCard() {
    GlassCard({
      kicker: 'TIMELINE / SAMPLES',
      title: '回放采样点',
      subtitle: '点击任意采样点跳转'
    }) {
      Column({ space: 0 }) {
        ForEach(this.replayVisiblePoints(), (point: InsightsTimelinePoint, index: number) => {
          Row({ space: 8 }) {
            Circle({ width: 6, height: 6 })
              .fill(statusMeta(point.status).color)
            Text(formatDateTime(point.time))
              .fontSize(9)
              .fontColor(COLORS.text)
              .layoutWeight(1)
            Text(statusMeta(point.status).zh)
              .fontSize(8)
              .fontColor(statusMeta(point.status).color)
            Text(`光照 ${formatNumber(point.light, 0)}%`)
              .fontSize(8)
              .fontColor(COLORS.dim)
          }
          .width('100%')
          .padding({ top: 7, bottom: 7 })
          .border({ width: { bottom: 1 }, color: '#0F94A3B8' })
          .onClick(() => {
            this.replayIndex = this.timelineIndex(point.time);
            this.replayActive = true;
            this.replayPlaying = false;
          })
        }, (point: InsightsTimelinePoint, index: number) => `${point.time}-${index}`)
      }
      .width('100%')
    }
  }

  @Builder
  buildDemoCard() {
    GlassCard({
      kicker: 'DEMO / SCENARIO',
      title: '演示控制台',
      subtitle: '通过真实 IoTDA 事件路径写入 Postgres，不是前端假状态'
    }) {
      Text('点击场景按钮，后端会生成一条设备属性事件，经过 Parse → Ingest 后写入数据库。')
        .fontSize(10)
        .fontColor(COLORS.muted)
        .lineHeight(16)

      Row({ space: 8 }) {
        Column({ space: 8 }) {
          this.buildDemoButton(DEMO_SCENARIOS[0])
          this.buildDemoButton(DEMO_SCENARIOS[2])
        }
        .layoutWeight(1)
        Column({ space: 8 }) {
          this.buildDemoButton(DEMO_SCENARIOS[1])
          this.buildDemoButton(DEMO_SCENARIOS[3])
        }
        .layoutWeight(1)
      }
      .width('100%')
      .alignItems(VerticalAlign.Top)

      Row({ space: 8 }) {
        Text('自动演示')
          .fontSize(11)
          .fontColor(COLORS.text)
        Text('每 5 秒循环注入')
          .fontSize(8)
          .fontColor(COLORS.dim)
        Blank()
        Toggle({ type: ToggleType.Switch, isOn: this.autoDemo })
          .selectedColor(COLORS.cyan)
          .onChange((isOn: boolean) => {
            this.autoDemo = isOn;
            this.syncDemoLoop();
          })
      }
      .width('100%')
      .padding(10)
      .backgroundColor('#0F94A3B8')
      .borderRadius(10)

      if (this.demoMessage.length > 0) {
        Text(this.demoMessage)
          .fontSize(9)
          .fontColor(this.demoMessage.includes('失败') ? '#FECDD3' : '#A7F3D0')
          .padding(8)
          .width('100%')
          .backgroundColor('#1494A3B8')
          .borderRadius(8)
      }
    }
  }

  @Builder
  buildDemoButton(scenario: DemoScenario) {
    Row({ space: 8 }) {
      SymbolGlyph(scenario.icon)
        .fontSize(18)
        .fontColor([scenario.color])
      Column({ space: 2 }) {
        Text(scenario.label)
          .fontSize(11)
          .fontColor(COLORS.text)
        Text(scenario.hint)
          .fontSize(7)
          .fontColor(COLORS.dim)
      }
      .layoutWeight(1)
      .alignItems(HorizontalAlign.Start)
    }
    .width('100%')
    .padding(9)
    .borderRadius(10)
    .backgroundColor('#0F94A3B8')
    .border({ width: 1, color: '#1494A3B8' })
    .opacity(this.demoBusy ? 0.55 : 1)
    .onClick(() => {
      this.runDemo(scenario.key);
    })
  }

  @Builder
  buildConnectionCard() {
    GlassCard({
      kicker: 'CONNECTION',
      title: '数据连接',
      subtitle: '真机通过局域网访问 Mac 上的 Go 服务'
    }) {
      Column({ space: 6 }) {
        Row({ space: 8 }) {
          Text('API Base URL')
            .fontSize(9)
            .fontColor(COLORS.dim)
            .width(92)
          Text(this.apiBaseUrl)
            .fontSize(9)
            .fontColor(COLORS.cyan)
            .layoutWeight(1)
            .maxLines(1)
            .textOverflow({ overflow: TextOverflow.Ellipsis })
        }
        Row({ space: 8 }) {
          Text('API Token')
            .fontSize(9)
            .fontColor(COLORS.dim)
            .width(92)
          Text(this.maskToken())
            .fontSize(9)
            .fontColor(COLORS.muted)
            .layoutWeight(1)
        }
        Row({ space: 8 }) {
          Button('打开设置')
            .fontSize(10)
            .fontColor(Color.Black)
            .backgroundColor(COLORS.cyan)
            .height(30)
            .borderRadius(15)
            .onClick(() => this.openSettings())
          Button('立即刷新')
            .fontSize(10)
            .fontColor(COLORS.cyan)
            .backgroundColor('#1422D3EE')
            .height(30)
            .borderRadius(15)
            .onClick(() => this.refresh())
        }
      }
      .width('100%')
      .alignItems(HorizontalAlign.Start)
    }
  }

  @Builder
  buildNativeCard() {
    GlassCard({
      kicker: 'HARMONYOS / NATIVE',
      title: '鸿蒙原生能力',
      subtitle: '不是 WebView，全部是 ArkUI 声明式组件'
    }) {
      this.buildNativeRow($r('sys.symbol.square_grid_2x2'), 'HdsNavigation + HdsTabs', '沉浸光感标题栏与悬浮页签')
      this.buildNativeRow($r('sys.symbol.picture'), 'PixelMap 实时画面', '带 Bearer Token 拉取 JPEG 并解码')
      this.buildNativeRow($r('sys.symbol.heart_badge_plus'), 'Vibrator 触感', '坐姿告警时给出轻振反馈')
      this.buildNativeRow($r('sys.symbol.gearshape'), 'Preferences 持久化', 'Base URL / Token 保存到本机')
    }
  }

  @Builder
  buildNativeRow(icon: Resource, title: string, detail: string) {
    Row({ space: 8 }) {
      SymbolGlyph(icon)
        .fontSize(16)
        .fontColor([COLORS.cyan])
      Column({ space: 2 }) {
        Text(title)
          .fontSize(10)
          .fontColor(COLORS.text)
        Text(detail)
          .fontSize(8)
          .fontColor(COLORS.dim)
      }
      .layoutWeight(1)
      .alignItems(HorizontalAlign.Start)
    }
    .width('100%')
    .padding({ top: 6, bottom: 6 })
    .border({ width: { bottom: 1 }, color: '#0F94A3B8' })
  }

  @Builder
  buildSettingsSheet() {
    Column({ space: 14 }) {
      Row() {
        Column({ space: 3 }) {
          Text('连接设置')
            .fontSize(19)
            .fontWeight(FontWeight.Medium)
            .fontColor(COLORS.text)
          Text('真机与 Mac 需在同一网络；Go 服务需监听 0.0.0.0:11294')
            .fontSize(9)
            .fontColor(COLORS.muted)
        }
        .layoutWeight(1)
        .alignItems(HorizontalAlign.Start)
        SymbolGlyph($r('sys.symbol.gearshape'))
          .fontSize(22)
          .fontColor([COLORS.cyan])
      }
      .width('100%')

      Column({ space: 6 }) {
        Text('API Base URL')
          .fontSize(10)
          .fontColor(COLORS.dim)
        TextInput({ text: this.editBaseUrl, placeholder: Api.DEFAULT_BASE_URL })
          .fontSize(12)
          .fontColor(COLORS.text)
          .placeholderColor(COLORS.dim)
          .backgroundColor('#0F94A3B8')
          .borderRadius(10)
          .height(42)
          .onChange((value: string) => {
            this.editBaseUrl = value;
          })
      }
      .width('100%')
      .alignItems(HorizontalAlign.Start)

      Column({ space: 6 }) {
        Text('API Token')
          .fontSize(10)
          .fontColor(COLORS.dim)
        TextInput({ text: this.editToken, placeholder: 'test-api-token' })
          .type(InputType.Password)
          .fontSize(12)
          .fontColor(COLORS.text)
          .placeholderColor(COLORS.dim)
          .backgroundColor('#0F94A3B8')
          .borderRadius(10)
          .height(42)
          .onChange((value: string) => {
            this.editToken = value;
          })
      }
      .width('100%')
      .alignItems(HorizontalAlign.Start)

      Text('提示：当前后端已开启 DEMO_ENABLED，可在“演示”页注入坐姿场景。')
        .fontSize(9)
        .fontColor(COLORS.dim)
        .lineHeight(14)

      Row({ space: 8 }) {
        Button('取消')
          .fontSize(11)
          .fontColor(COLORS.muted)
          .backgroundColor('#0F94A3B8')
          .height(34)
          .borderRadius(17)
          .layoutWeight(1)
          .onClick(() => {
            this.showSettings = false;
          })
        Button('保存并刷新')
          .fontSize(11)
          .fontColor(Color.Black)
          .backgroundColor(COLORS.cyan)
          .height(34)
          .borderRadius(17)
          .layoutWeight(1)
          .onClick(() => this.saveConnection())
      }
      .width('100%')
    }
    .width('100%')
    .padding({ left: 18, right: 18, top: 10, bottom: 18 })
    .alignItems(HorizontalAlign.Start)
  }

  private async load(silent: boolean): Promise<void> {
    if (this.inFlight) {
      return;
    }
    this.inFlight = true;
    if (!silent) {
      this.loading = true;
      this.refreshing = true;
    }
    try {
      const devices: Device[] = await Api.listDevices();
      this.devices = devices;
      if (devices.length === 0) {
        this.selectedDeviceId = '';
        this.selectedDeviceIndex = 0;
        this.latest = [];
        this.lightPoints = [];
        this.rssiPoints = [];
        this.heapPoints = [];
        this.postureRows = [];
        this.events = [];
        this.insights = null;
        this.frames = [];
        this.selectedFrameId = 0;
        this.messages = [];
        this.messageError = '';
        this.aiReport = null;
        this.autoReportDeviceId = '';
        this.errorMessage = '';
        return;
      }

      let index: number = this.devices.findIndex((device: Device) => device.device_id === this.selectedDeviceId);
      if (index < 0) {
        index = this.devices.findIndex((device: Device) => device.device_id.includes('hbot'));
      }
      if (index < 0) {
        index = 0;
      }
      const device: Device = this.devices[index];
      if (device.device_id !== this.selectedDeviceId) {
        this.aiReport = null;
        this.autoReportDeviceId = '';
        this.replayActive = false;
        this.replayPlaying = false;
        this.replayIndex = 0;
      }
      this.selectedDeviceIndex = index;
      this.selectedDeviceId = device.device_id;

      const range: RangeParams = Api.rangeParams(this.activeRange);
      const latest: TelemetryRow[] = await Api.getLatest(device.device_id);
      const light: SeriesPoint[] = await Api.getSeries(device.device_id, 'Environment', 'light',
        range.from, range.to, range.bucket);
      const rssi: SeriesPoint[] = await Api.getSeries(device.device_id, 'DeviceStatus', 'rssi',
        range.from, range.to, range.bucket);
      const heap: SeriesPoint[] = await Api.getSeries(device.device_id, 'DeviceStatus', 'freeHeap',
        range.from, range.to, range.bucket);
      const posture: TelemetryRow[] = await Api.getTelemetry(device.device_id, 'Posture',
        'postureStatus', range.from, range.to, 240);
      const events: EventRow[] = await Api.getEvents(device.device_id, 14);
      const insights: Insights = await Api.getInsights(device.device_id, 24);
      const frames: PostureFrame[] = await Api.listFrames(device.device_id, 8);
      let messages: Message[] = [];
      let messagesError: string = '';
      try {
        messages = await Api.listMessages(device.device_id, 30);
      } catch (error) {
        messages = [];
        messagesError = messageOf(error);
      }

      this.latest = latest;
      this.lightPoints = this.seriesToChart(light);
      this.rssiPoints = this.seriesToChart(rssi);
      this.heapPoints = this.seriesToChart(heap);
      this.postureRows = posture;
      this.events = events;
      this.insights = insights;
      this.frames = frames;
      this.messages = messages;
      this.messageError = messagesError;
      this.lastUpdated = new Date().toISOString();
      this.errorMessage = '';

      if (frames.length === 0) {
        this.selectedFrameId = 0;
        if (this.framePixel !== undefined) {
          this.framePixel.release();
          this.framePixel = undefined;
        }
      } else {
        const current: PostureFrame | undefined =
          frames.find((frame: PostureFrame) => frame.id === this.selectedFrameId);
        if (current === undefined) {
          this.selectedFrameId = frames[0].id;
          this.loadFrame(frames[0].id);
        } else if (this.framePixel === undefined && this.loadedFrameId !== current.id) {
          this.loadFrame(current.id);
        }
      }

      if (insights.has_data && this.aiReport === null && this.autoReportDeviceId !== device.device_id) {
        this.autoReportDeviceId = device.device_id;
        this.generateReport();
      }
      if (this.replayIndex >= insights.timeline.length && insights.timeline.length > 0) {
        this.replayIndex = insights.timeline.length - 1;
      }
    } catch (error) {
      this.errorMessage = messageOf(error);
    } finally {
      this.loading = false;
      this.refreshing = false;
      this.inFlight = false;
    }
  }

  private async loadFrame(frameId: number): Promise<void> {
    if (frameId <= 0) {
      return;
    }
    this.frameLoading = true;
    this.frameError = '';
    try {
      const pixel: image.PixelMap = await Api.getFramePixelMap(frameId);
      if (this.framePixel !== undefined) {
        this.framePixel.release();
      }
      this.framePixel = pixel;
      this.loadedFrameId = frameId;
      this.selectedFrameId = frameId;
    } catch (error) {
      this.frameError = messageOf(error);
    } finally {
      this.frameLoading = false;
    }
  }

  private async generateReport(): Promise<void> {
    if (this.selectedDeviceId.length === 0 || this.aiLoading) {
      return;
    }
    this.aiLoading = true;
    this.aiError = '';
    try {
      this.aiReport = await Api.generateAIReport(this.selectedDeviceId, 24);
    } catch (error) {
      this.aiError = messageOf(error);
    } finally {
      this.aiLoading = false;
    }
  }

  private async sendMessage(): Promise<void> {
    const draft: string = this.messageDraft.trim();
    if (draft.length === 0 || this.selectedDeviceId.length === 0 || this.messageSending) {
      return;
    }
    this.messageSending = true;
    this.messageError = '';
    try {
      const authorName: string = this.messageRole === 'parent' ? '妈妈' : '小宇';
      await Api.sendMessage(this.selectedDeviceId, this.messageRole, authorName, draft);
      this.messageDraft = '';
      this.messages = await Api.listMessages(this.selectedDeviceId, 30);
    } catch (error) {
      this.messageError = messageOf(error);
    } finally {
      this.messageSending = false;
    }
  }

  private async runDemo(status: string): Promise<void> {
    if (this.demoBusy || this.selectedDeviceId.length === 0) {
      return;
    }
    this.demoBusy = true;
    this.demoMessage = '';
    try {
      const result: DemoResult = await Api.runDemoScenario(this.selectedDeviceId, status);
      this.demoMessage = `已注入：${result.summary} · ${formatClock(result.event_time)}`;
      this.toast(this.demoMessage);
      if (status === 'bad' || status === 'warning') {
        this.vibrate();
      }
      await this.load(true);
    } catch (error) {
      this.demoMessage = messageOf(error);
    } finally {
      this.demoBusy = false;
    }
  }

  private syncDemoLoop(): void {
    if (!this.autoDemo) {
      this.stopDemoLoop();
      return;
    }
    if (this.demoTimer >= 0) {
      return;
    }
    this.demoIndex = 0;
    this.demoTimer = setInterval(() => {
      if (!this.autoDemo || this.demoBusy) {
        return;
      }
      const sequence: string[] = ['good', 'warning', 'bad', 'good', 'away'];
      const status: string = sequence[this.demoIndex % sequence.length];
      this.demoIndex += 1;
      this.runDemo(status);
    }, 5000);
  }

  private stopDemoLoop(): void {
    if (this.demoTimer >= 0) {
      clearInterval(this.demoTimer);
      this.demoTimer = -1;
    }
  }

  private startReplay(): void {
    if (this.timeline().length === 0) {
      return;
    }
    this.replayActive = true;
    this.replayPlaying = true;
    this.replayIndex = 0;
  }

  private toggleReplayPlaying(): void {
    if (!this.replayActive) {
      this.startReplay();
      return;
    }
    if (this.replayIndex >= this.timeline().length - 1) {
      this.replayIndex = 0;
    }
    this.replayPlaying = !this.replayPlaying;
  }

  private exitReplay(): void {
    this.replayActive = false;
    this.replayPlaying = false;
    this.replayIndex = 0;
  }

  private refresh(): void {
    this.load(false);
  }

  private changeRange(range: string): void {
    if (this.activeRange === range) {
      return;
    }
    this.activeRange = range;
    this.load(false);
  }

  private selectDevice(index: number): void {
    if (index < 0 || index >= this.devices.length) {
      return;
    }
    const nextId: string = this.devices[index].device_id;
    if (nextId === this.selectedDeviceId) {
      return;
    }
    this.selectedDeviceIndex = index;
    this.selectedDeviceId = nextId;
    this.aiReport = null;
    this.aiError = '';
    this.autoReportDeviceId = '';
    this.exitReplay();
    this.load(false);
  }

  private selectFrame(frameId: number): void {
    if (frameId === this.selectedFrameId && this.framePixel !== undefined) {
      return;
    }
    this.selectedFrameId = frameId;
    this.loadFrame(frameId);
  }

  private openSettings(): void {
    this.editBaseUrl = this.apiBaseUrl;
    this.editToken = this.apiToken;
    this.showSettings = true;
  }

  private saveConnection(): void {
    const host: common.Context | undefined = this.getUIContext().getHostContext();
    if (host === undefined) {
      this.toast('无法获取应用上下文');
      return;
    }
    Api.saveSettings(host, this.editBaseUrl, this.editToken);
    this.showSettings = false;
    this.aiReport = null;
    this.autoReportDeviceId = '';
    this.toast('已保存，正在刷新数据');
    this.load(false);
  }

  private toast(message: string): void {
    const options: promptAction.ShowToastOptions = { message: message };
    try {
      options.systemMaterial = new uiMaterial.ImmersiveMaterial({
        style: uiMaterial.ImmersiveStyle.THIN
      });
    } catch (error) {
      // Older devices use the default toast style.
    }
    try {
      this.getUIContext().getPromptAction().showToast(options);
    } catch (error) {
      // Toast is best-effort; the page state has already updated.
    }
  }

  private vibrate(): void {
    try {
      vibrator.startVibration({
        type: 'preset',
        effectId: 'haptic.effect.soft',
        count: 1
      }, {
        usage: 'touch'
      }).catch((error: BusinessError) => {
        // Haptics are best-effort.
      });
    } catch (error) {
      // Haptics are best-effort.
    }
  }

  private seriesToChart(series: SeriesPoint[]): ChartPoint[] {
    const points: ChartPoint[] = [];
    for (let i = 0; i < series.length; i++) {
      const avg: number | null | undefined = series[i].avg;
      if (avg !== null && avg !== undefined && Number.isFinite(avg)) {
        points.push({ time: series[i].bucket, value: avg });
      }
    }
    return points;
  }

  private latestValue(service: string, property: string): Object | null | undefined {
    for (let i = 0; i < this.latest.length; i++) {
      if (this.latest[i].service_id === service && this.latest[i].property === property) {
        return this.latest[i].value;
      }
    }
    return undefined;
  }

  private latestNumber(service: string, property: string, fallback: number): number {
    const value: Object | null | undefined = this.latestValue(service, property);
    if (value === undefined || value === null) {
      return fallback;
    }
    if (typeof value === 'number') {
      return value as number;
    }
    const parsed: number = Number(`${value}`);
    return Number.isNaN(parsed) ? fallback : parsed;
  }

  private latestText(service: string, property: string, fallback: string): string {
    const value: Object | null | undefined = this.latestValue(service, property);
    if (value === undefined || value === null) {
      return fallback;
    }
    return `${value}`;
  }

  private selectedDevice(): Device | undefined {
    return this.devices.find((device: Device) => device.device_id === this.selectedDeviceId);
  }

  private selectedDeviceLabel(): string {
    const device: Device | undefined = this.selectedDevice();
    if (device === undefined) {
      return this.devices.length === 0 ? '等待设备' : '孩子的书桌';
    }
    return this.deviceLabel(device);
  }

  private deviceLabel(device: Device): string {
    if (device.device_id.indexOf('hbot') >= 0) {
      return '孩子的书桌';
    }
    if (device.node_id !== undefined && device.node_id !== null &&
      device.node_id.length > 0 && device.node_id !== device.device_id) {
      return device.node_id;
    }
    return device.device_id;
  }

  private deviceOptions(): SelectOption[] {
    const options: SelectOption[] = [];
    for (let i = 0; i < this.devices.length; i++) {
      options.push({ value: this.deviceLabel(this.devices[i]) });
    }
    return options;
  }

  private timeline(): InsightsTimelinePoint[] {
    return this.insights === null ? [] : (this.insights as Insights).timeline;
  }

  private recentTimeline(): InsightsTimelinePoint[] {
    const all: InsightsTimelinePoint[] = this.timeline();
    return all.length <= 60 ? all : all.slice(all.length - 60);
  }

  private heatmapCells(): InsightsHeatmapCell[] {
    return this.insights === null ? [] : (this.insights as Insights).heatmap;
  }

  private timelineIndex(time: string): number {
    const all: InsightsTimelinePoint[] = this.timeline();
    for (let i = 0; i < all.length; i++) {
      if (all[i].time === time) {
        return i;
      }
    }
    return 0;
  }

  private replayDisplayIndex(): number {
    const length: number = this.timeline().length;
    if (length === 0) {
      return 0;
    }
    if (this.replayActive) {
      return clamp(this.replayIndex, 0, length - 1);
    }
    return length - 1;
  }

  private replayPoint(): InsightsTimelinePoint | null {
    const all: InsightsTimelinePoint[] = this.timeline();
    if (all.length === 0) {
      return null;
    }
    const index: number = clamp(this.replayIndex, 0, all.length - 1);
    return all[index];
  }

  private replayCurrentTime(): string {
    const all: InsightsTimelinePoint[] = this.timeline();
    if (all.length === 0) {
      return '';
    }
    const index: number = this.replayActive ? clamp(this.replayIndex, 0, all.length - 1) : all.length - 1;
    return all[index].time;
  }

  private replayStatus(): string {
    if (this.replayActive) {
      const point: InsightsTimelinePoint | null = this.replayPoint();
      return point === null ? 'unknown' : statusOf(point.status);
    }
    return this.displayStatus();
  }

  private replayLight(): number {
    const point: InsightsTimelinePoint | null = this.replayPoint();
    return point === null ? this.displayLight() : point.light;
  }

  private replayPerson(): boolean {
    if (!this.replayActive) {
      return this.displayPerson();
    }
    const point: InsightsTimelinePoint | null = this.replayPoint();
    return point === null ? false : point.person;
  }

  private replayAlert(): boolean {
    if (!this.replayActive) {
      return this.displayAlert();
    }
    const point: InsightsTimelinePoint | null = this.replayPoint();
    return point === null ? false : point.alert;
  }

  private replayLightPoints(): ChartPoint[] {
    const all: InsightsTimelinePoint[] = this.timeline();
    const points: ChartPoint[] = [];
    for (let i = 0; i < all.length; i++) {
      points.push({ time: all[i].time, value: all[i].light });
    }
    return points;
  }

  private replayVisiblePoints(): InsightsTimelinePoint[] {
    const all: InsightsTimelinePoint[] = this.timeline();
    if (all.length === 0) {
      return [];
    }
    const center: number = this.replayDisplayIndex();
    const start: number = Math.max(0, center - 5);
    const end: number = Math.min(all.length, center + 6);
    return all.slice(start, end);
  }

  private displayStatus(): string {
    if (this.replayActive) {
      const point: InsightsTimelinePoint | null = this.replayPoint();
      return point === null ? 'unknown' : statusOf(point.status);
    }
    return statusOf(this.latestText('Posture', 'postureStatus', 'unknown'));
  }

  private displaySummary(): string {
    if (this.replayActive) {
      const point: InsightsTimelinePoint | null = this.replayPoint();
      if (point !== null) {
        return `${statusMeta(statusOf(point.status)).zh} · 历史回放`;
      }
    }
    return this.latestText('Posture', 'postureSummary', statusMeta(this.displayStatus()).zh);
  }

  private displaySedentary(): boolean {
    return this.displaySummary().indexOf('久坐') >= 0;
  }

  private displayPerson(): boolean {
    if (this.replayActive) {
      const point: InsightsTimelinePoint | null = this.replayPoint();
      return point === null ? false : point.person;
    }
    return this.latestNumber('Posture', 'personDetected', 0) === 1;
  }

  private displayAlert(): boolean {
    if (this.replayActive) {
      const point: InsightsTimelinePoint | null = this.replayPoint();
      return point === null ? false : point.alert;
    }
    const status: string = this.displayStatus();
    if (status === 'bad' || status === 'warning') {
      return true;
    }
    return this.latestNumber('Posture', 'alert', 0) === 1;
  }

  private displayIssueCount(): number {
    if (this.replayActive) {
      return this.displayAlert() ? 1 : 0;
    }
    const count: number = this.latestNumber('Posture', 'issueCount', 0);
    if (count > 0) {
      return count;
    }
    const status: string = this.displayStatus();
    if (status === 'bad') {
      return 2;
    }
    if (status === 'warning') {
      return 1;
    }
    return 0;
  }

  private displaySequence(): number {
    if (this.replayActive) {
      return 0;
    }
    return this.latestNumber('Posture', 'lastSeq', 0);
  }

  private displayLastSeen(): string {
    if (this.replayActive) {
      const point: InsightsTimelinePoint | null = this.replayPoint();
      return point === null ? '' : point.time;
    }
    const device: Device | undefined = this.selectedDevice();
    return device === undefined ? '' : device.last_seen_at;
  }

  private displayLight(): number {
    if (this.replayActive) {
      const point: InsightsTimelinePoint | null = this.replayPoint();
      return point === null ? 0 : point.light;
    }
    return this.latestNumber('Environment', 'light', 0);
  }

  private displaySound(): number {
    if (this.replayActive) {
      const point: InsightsTimelinePoint | null = this.replayPoint();
      return point !== null && point.sound ? 1 : 0;
    }
    return this.latestNumber('Environment', 'sound', 0);
  }

  private score(): number {
    if (this.aiReport !== null) {
      return (this.aiReport as AIReport).score;
    }
    return this.insights === null ? 0 : (this.insights as Insights).health_score;
  }

  private scoreLevelLabel(): string {
    if (this.aiReport !== null) {
      return levelLabel((this.aiReport as AIReport).level);
    }
    if (this.score() >= 85) {
      return '优秀';
    }
    if (this.score() >= 70) {
      return '良好';
    }
    if (this.score() >= 55) {
      return '需关注';
    }
    return '需干预';
  }

  private summaryPercent(): string {
    if (this.insights === null) {
      return '—';
    }
    return formatPercent((this.insights as Insights).summary.good_rate);
  }

  private summaryAlerts(): string {
    if (this.insights === null) {
      return '—';
    }
    return `${(this.insights as Insights).summary.alert_events} 次`;
  }

  private summaryStreak(): string {
    if (this.insights === null) {
      return '—';
    }
    return `${(this.insights as Insights).summary.longest_bad_streak_minutes} 分`;
  }

  private summaryLight(): string {
    if (this.insights === null) {
      return '—';
    }
    return `${formatNumber((this.insights as Insights).summary.avg_light, 0)}%`;
  }

  private summaryGoodHint(): string {
    if (this.insights === null) {
      return '等待统计';
    }
    return `${(this.insights as Insights).summary.good} 个良好样本`;
  }

  private lightAverage(): string {
    if (this.lightPoints.length === 0) {
      return '—';
    }
    let sum: number = 0;
    for (let i = 0; i < this.lightPoints.length; i++) {
      sum += this.lightPoints[i].value;
    }
    return `${formatNumber(sum / this.lightPoints.length, 0)}%`;
  }

  private lightPeak(): string {
    if (this.lightPoints.length === 0) {
      return '—';
    }
    let peak: number = this.lightPoints[0].value;
    for (let i = 1; i < this.lightPoints.length; i++) {
      if (this.lightPoints[i].value > peak) {
        peak = this.lightPoints[i].value;
      }
    }
    return `${formatNumber(peak, 0)}%`;
  }

  private mixItems(): MixItem[] {
    if (this.insights === null) {
      return [];
    }
    const summary: InsightsSummary = (this.insights as Insights).summary;
    return [
      { key: 'good', label: '坐姿良好', count: summary.good, color: COLORS.emerald },
      { key: 'warning', label: '坐姿偏移', count: summary.warning, color: COLORS.amber },
      { key: 'bad', label: '坐姿异常', count: summary.bad, color: COLORS.rose },
      { key: 'unknown', label: '暂时离开', count: summary.unknown, color: COLORS.slate }
    ];
  }

  private mixPercent(item: MixItem): string {
    let total: number = 0;
    const items: MixItem[] = this.mixItems();
    for (let i = 0; i < items.length; i++) {
      total += items[i].count;
    }
    return total === 0 ? '0%' : `${Math.round((item.count / total) * 100)}%`;
  }

  private topRiskText(): string {
    if (this.insights === null || (this.insights as Insights).risk_windows.length === 0) {
      return '暂未发现明显的高风险时段';
    }
    const window: InsightsRiskWindow = (this.insights as Insights).risk_windows[0];
    return `${weekdayName(window.weekday)} ${String(window.hour).padStart(2, '0')}:00–${String((window.hour + 1) % 24).padStart(2, '0')}:00 · 告警率 ${formatPercent(window.alert_rate)}`;
  }

  private selectedFrame(): PostureFrame | undefined {
    return this.frames.find((frame: PostureFrame) => frame.id === this.selectedFrameId);
  }

  private frameStatus(frame: PostureFrame): string {
    return statusOf(frame.status === undefined || frame.status === null ? 'unknown' : frame.status);
  }

  private eventTitle(event: EventRow): string {
    if (event.resource === 'device.property') {
      return '坐姿 / 环境属性上报';
    }
    if (event.resource === 'device.status') {
      return `设备状态 · ${event.event ?? 'update'}`;
    }
    if (event.resource === 'device.message') {
      return '设备消息';
    }
    return event.resource;
  }

  private eventColor(event: EventRow): string {
    if (event.resource === 'device.status') {
      return COLORS.emerald;
    }
    if (event.resource === 'device.message') {
      return COLORS.violet;
    }
    return COLORS.cyan;
  }

  private maskToken(): string {
    if (this.apiToken.length <= 6) {
      return this.apiToken.length === 0 ? '未设置' : '••••••';
    }
    return `${this.apiToken.substring(0, 3)}••••${this.apiToken.substring(this.apiToken.length - 3)}`;
  }

  private formatUptime(seconds: number): string {
    if (!Number.isFinite(seconds) || seconds <= 0) {
      return '—';
    }
    const hours: number = Math.floor(seconds / 3600);
    const minutes: number = Math.floor((seconds % 3600) / 60);
    return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
  }
}
