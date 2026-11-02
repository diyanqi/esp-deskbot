const api = require('../../utils/api');
const T = require('../../utils/theme');

const RANGES = ['1H', '6H', '24H', '7D'];
const TABS = [
  { key: 'overview', label: '总览', icon: '▦' },
  { key: 'report', label: 'AI 报告', icon: '▤' },
  { key: 'messages', label: '留言', icon: '✉' },
  { key: 'replay', label: '时间回放', icon: '◷' },
  { key: 'demo', label: '演示', icon: '✦' }
];
const METRIC_OPTIONS = [
  { key: 'alert', label: '告警率', color: '#fb7185' },
  { key: 'good', label: '良好率', color: '#34d399' },
  { key: 'light', label: '光照', color: '#fbbf24' }
];
const DEMO_SCENARIOS = [
  { key: 'good', label: '良好坐姿', hint: '绿色 · 无提醒', icon: '✓', color: '#34d399' },
  { key: 'warning', label: '轻微偏移', hint: '黄色 · 一次提醒', icon: '!', color: '#fbbf24' },
  { key: 'bad', label: '坐姿告警', hint: '红色 · 强提醒', icon: '⚠', color: '#fb7185' },
  { key: 'away', label: '离开书桌', hint: '灰色 · 无人', icon: '○', color: '#94a3b8' }
];
const DEFAULT_DISPLAY = {
  status: 'unknown',
  statusLabel: 'AWAY',
  statusZh: '暂时离开',
  statusColor: '#94a3b8',
  statusSoft: 'rgba(148,163,184,0.12)',
  faceClass: 'face-unknown',
  summary: '等待孩子的坐姿数据',
  person: false,
  alert: false,
  issue: 0,
  seq: 0,
  lastSeen: '',
  lastSeenLabel: '—',
  light: 0,
  sound: 0,
  sedentary: false
};

Page({
  data: {
    statusBarHeight: 20,
    navHeight: 64,
    capsuleRight: 12,
    online: true,
    activeTab: 'overview',
    tabs: TABS,
    ranges: RANGES,
    range: '24H',
    autoRefresh: true,
    devices: [],
    deviceLabels: [],
    deviceIndex: 0,
    selectedDeviceId: '',
    deviceLabel: '等待设备',
    latest: [],
    insights: null,
    lightPoints: [],
    lightValues: [],
    rssiPoints: [],
    heapPoints: [],
    postureRows: [],
    events: [],
    eventsDisplay: [],
    loading: true,
    refreshing: false,
    error: '',
    lastUpdated: '',
    updatedAgo: '等待同步',
    uptimeText: '—',
    firmwareText: '—',
    reportInterval: 0,
    display: DEFAULT_DISPLAY,
    score: 0,
    scoreColor: '#22d3ee',
    levelLabel: '等待数据',
    aiHeadline: '今天也在认真守护成长',
    aiSummary: '评分综合坐姿良好率、提醒频率、光照舒适度与在座时长。',
    metrics: [],
    kpis: [],
    lightNow: '—',
    lightAvg: '—',
    lightPeak: '—',
    mixItems: [],
    timeline: [],
    timelineSegments: [],
    frames: [],
    framesView: [],
    selectedFrameId: 0,
    frameImage: '',
    frameLoading: false,
    frameError: '',
    frameStatus: 'unknown',
    frameStatusLabel: '',
    frameStatusZh: '',
    frameStatusColor: '#94a3b8',
    frameSummary: '',
    frameTime: '—',
    messages: [],
    messagesDisplay: [],
    messageDraft: '',
    messageRole: 'parent',
    messageSending: false,
    messageError: '',
    aiReport: null,
    aiFindings: [],
    aiRiskWindows: [],
    aiSuggestions: [],
    aiSourceLabel: '',
    aiLoading: false,
    aiError: '',
    heatMetric: 'alert',
    metricOptions: METRIC_OPTIONS,
    heatmapCells: [],
    riskText: '暂未发现明显的高风险时段',
    replayActive: false,
    replayPlaying: false,
    replayIndex: 0,
    replayDisplayIndex: 0,
    replayMax: 0,
    replayTime: '—',
    replayStatus: 'unknown',
    replayStatusLabel: '',
    replayStatusColor: '#94a3b8',
    replayLight: 0,
    replayPerson: false,
    replayAlert: false,
    replayValues: [],
    replaySegments: [],
    replayVisible: [],
    demoScenarios: DEMO_SCENARIOS,
    demoBusy: false,
    demoMessage: '',
    autoDemo: false,
    showSettings: false,
    editBaseUrl: '',
    editToken: '',
    apiBaseUrl: '',
    tokenMasked: '未设置',
    nativeItems: [
      { icon: '◷', title: '自定义导航栏', detail: '适配状态栏与胶囊按钮' },
      { icon: '▦', title: 'Canvas 2D 图表', detail: '原生绘制光照/回放曲线' },
      { icon: '▣', title: 'downloadFile + image', detail: '带 Bearer Token 拉取标注图' },
      { icon: '✧', title: 'Vibrate / Toast / Storage', detail: '原生触感、反馈与本地设置' }
    ]
  },

  onLoad() {
    this.initNavigation();
    const config = api.getConfig();
    this.setData({
      apiBaseUrl: config.baseUrl,
      editBaseUrl: config.baseUrl,
      editToken: config.token,
      tokenMasked: T.maskToken(config.token)
    });
    this.requestNetworkState();
    if (wx.onNetworkStatusChange) {
      wx.onNetworkStatusChange((result) => {
        this.setData({ online: !!result.isConnected });
      });
    }
    this.load(false);
  },

  onShow() {
    this.startTimers();
    if (this._loadedOnce) {
      this.load(true);
    }
  },

  onHide() {
    this.stopTimers();
  },

  onUnload() {
    this.stopTimers();
    if (this.data.frameImage) {
      // The temp file is managed by the Mini Program runtime.
    }
  },

  onShareAppMessage() {
    return {
      title: '成长守护 · 家长端坐姿健康',
      path: '/pages/index/index'
    };
  },

  initNavigation() {
    let system = null;
    try {
      system = wx.getWindowInfo ? wx.getWindowInfo() : wx.getSystemInfoSync();
    } catch (error) {
      system = wx.getSystemInfoSync();
    }
    const statusBarHeight = system.statusBarHeight || 20;
    let navHeight = statusBarHeight + 44;
    let capsuleRight = 12;
    try {
      const menu = wx.getMenuButtonBoundingClientRect();
      if (menu && menu.height > 0 && menu.top > 0) {
        const gap = Math.max(4, menu.top - statusBarHeight);
        navHeight = statusBarHeight + menu.height + gap * 2;
        capsuleRight = Math.max(12, (system.windowWidth || 375) - menu.left + 8);
      }
    } catch (error) {
      // Use the default nav height.
    }
    this.setData({ statusBarHeight, navHeight, capsuleRight });
  },

  requestNetworkState() {
    try {
      wx.getNetworkType({
        success: (result) => {
          this.setData({ online: result.networkType !== 'none' });
        }
      });
    } catch (error) {
      // Ignore.
    }
  },

  startTimers() {
    this.stopTimers();
    this._autoTimer = setInterval(() => {
      if (this.data.autoRefresh && !this._loading) {
        this.load(true);
      }
    }, 5000);
    this._replayTimer = setInterval(() => {
      if (!this.data.replayActive || !this.data.replayPlaying) {
        return;
      }
      const length = (this.data.timeline || []).length;
      if (length === 0) {
        return;
      }
      if (this.data.replayIndex >= length - 1) {
        this.setData({ replayPlaying: false }, () => this.applyReplay());
        return;
      }
      this.setData({ replayIndex: this.data.replayIndex + 1 }, () => this.applyReplay());
    }, 220);
    if (this.data.autoDemo) {
      this.syncDemoTimer();
    }
  },

  stopTimers() {
    if (this._autoTimer) {
      clearInterval(this._autoTimer);
      this._autoTimer = null;
    }
    if (this._replayTimer) {
      clearInterval(this._replayTimer);
      this._replayTimer = null;
    }
    if (this._demoTimer) {
      clearInterval(this._demoTimer);
      this._demoTimer = null;
    }
  },

  syncDemoTimer() {
    if (this._demoTimer) {
      clearInterval(this._demoTimer);
      this._demoTimer = null;
    }
    if (!this.data.autoDemo) {
      return;
    }
    let index = 0;
    const sequence = ['good', 'warning', 'bad', 'good', 'away'];
    this._demoTimer = setInterval(() => {
      if (!this.data.autoDemo || this.data.demoBusy) {
        return;
      }
      const status = sequence[index % sequence.length];
      index += 1;
      this.runDemo(status);
    }, 5000);
  },

  load(silent) {
    if (this._loading) {
      return Promise.resolve();
    }
    this._loading = true;
    if (!silent) {
      this.setData({
        loading: this.data.devices.length === 0,
        refreshing: true,
        error: ''
      });
    }

    return api
      .getDevices()
      .then((devices) => {
        if (!devices || devices.length === 0) {
          this.setData({
            devices: [],
            deviceLabels: [],
            deviceIndex: 0,
            selectedDeviceId: '',
            deviceLabel: '等待设备',
            latest: [],
            insights: null,
            lightPoints: [],
            lightValues: [],
            postureRows: [],
            events: [],
            eventsDisplay: [],
            timeline: [],
            timelineSegments: [],
            frames: [],
            framesView: [],
            frameImage: '',
            messages: [],
            messagesDisplay: [],
            loading: false,
            refreshing: false,
            error: ''
          });
          this._loadedOnce = true;
          return null;
        }

        let index = devices.findIndex((device) => device.device_id === this.data.selectedDeviceId);
        if (index < 0) {
          index = devices.findIndex((device) => device.device_id.indexOf('hbot') >= 0);
        }
        if (index < 0) {
          index = 0;
        }
        const device = devices[index];
        const labels = devices.map((item) => this.deviceLabel(item));
        const deviceChanged = device.device_id !== this.data.selectedDeviceId;
        if (deviceChanged) {
          this.setData({
            aiReport: null,
            aiFindings: [],
            aiRiskWindows: [],
            aiSuggestions: [],
            aiError: '',
            replayActive: false,
            replayPlaying: false,
            replayIndex: 0
          });
          this.autoReportDeviceId = '';
        }
        this.setData({
          devices,
          deviceLabels: labels,
          deviceIndex: index,
          selectedDeviceId: device.device_id,
          deviceLabel: labels[index]
        });

        const range = api.rangeParams(this.data.range);
        return Promise.all([
          api.getLatest(device.device_id),
          api.getSeries(device.device_id, 'Environment', 'light', range.from, range.to, range.bucket),
          api.getSeries(device.device_id, 'DeviceStatus', 'rssi', range.from, range.to, range.bucket),
          api.getSeries(device.device_id, 'DeviceStatus', 'freeHeap', range.from, range.to, range.bucket),
          api.getTelemetry(device.device_id, 'Posture', 'postureStatus', range.from, range.to, 240),
          api.getEvents(device.device_id, 14),
          api.getInsights(device.device_id, 24),
          api.listFrames(device.device_id, 8),
          api.getMessages(device.device_id, 30).catch(() => [])
        ]).then((result) => {
          const latest = result[0] || [];
          const light = result[1] || [];
          const rssi = result[2] || [];
          const heap = result[3] || [];
          const posture = result[4] || [];
          const events = result[5] || [];
          const insights = result[6];
          const frames = result[7] || [];
          const messages = result[8] || [];
          this.setData(
            {
              latest,
              insights,
              lightPoints: this.seriesToPoints(light),
              rssiPoints: this.seriesToPoints(rssi),
              heapPoints: this.seriesToPoints(heap),
              postureRows: posture,
              events,
              frames,
              messages,
              messagesDisplay: this.buildMessagesView(messages),
              lastUpdated: new Date().toISOString(),
              loading: false,
              refreshing: false,
              error: ''
            },
            () => {
              this.applyAll();
              this.autoSelectFrame(frames);
              if (
                insights &&
                insights.has_data &&
                !this.data.aiReport &&
                this.autoReportDeviceId !== device.device_id
              ) {
                this.autoReportDeviceId = device.device_id;
                this.generateReport();
              }
            }
          );
        });
      })
      .catch((error) => {
        this.setData({
          loading: false,
          refreshing: false,
          error: error.message || `${error}`
        });
      })
      .then(() => {
        this._loading = false;
        this._loadedOnce = true;
      });
  },

  seriesToPoints(series) {
    const points = [];
    (series || []).forEach((item) => {
      if (typeof item.avg === 'number' && Number.isFinite(item.avg)) {
        points.push({ time: item.bucket, value: item.avg });
      }
    });
    return points;
  },

  applyAll() {
    const latest = this.data.latest || [];
    const insights = this.data.insights;
    const aiReport = this.data.aiReport;
    const display = this.computeDisplay(latest, insights, aiReport);
    const score = aiReport ? aiReport.score : insights ? insights.health_score : 0;
    const scoreColor = T.scoreColor(score);
    const level = aiReport ? T.levelLabel(aiReport.level) : score >= 85 ? '优秀' : score >= 70 ? '良好' : score >= 55 ? '需关注' : '需干预';
    const lightValues = this.data.lightPoints.map((point) => point.value);
    const lightNow = display.light;
    let lightSum = 0;
    let lightPeak = 0;
    lightValues.forEach((value) => {
      lightSum += value;
      if (value > lightPeak) {
        lightPeak = value;
      }
    });
    const lightAvg = lightValues.length ? lightSum / lightValues.length : 0;
    const metrics = this.buildMetrics(latest, display);
    const kpis = this.buildKpis(insights);
    const mixItems = this.buildMix(insights);
    const timeline = insights ? insights.timeline || [] : [];
    const timelineSegments = this.buildTimelineSegments(timeline);
    const eventsDisplay = this.buildEvents(this.data.events);
    const framesView = this.buildFramesView(this.data.frames);
    const heatmapCells = insights ? insights.heatmap || [] : [];
    const replay = this.computeReplay(display);
    const replayStatics = this.buildReplayStatics(timeline);
    const frame = this.computeFrame();
    const uptimeText = this.formatUptime(
      this.latestNumber(latest, 'DeviceStatus', 'uptime', 0)
    );
    const firmwareText = this.latestText(latest, 'DeviceStatus', 'firmware', '—');
    const reportInterval = this.latestNumber(latest, 'DeviceStatus', 'reportInterval', 0);

    this.setData(
      Object.assign(
        {
          display,
          score,
          scoreColor,
          levelLabel: level,
          aiHeadline: aiReport ? aiReport.headline : '今天也在认真守护成长',
          aiSummary: aiReport ? aiReport.summary : '评分综合坐姿良好率、提醒频率、光照舒适度与在座时长。',
          lightValues,
          lightNow: `${T.formatNumber(lightNow, 0)}%`,
          lightAvg: lightValues.length ? `${T.formatNumber(lightAvg, 0)}%` : '—',
          lightPeak: lightValues.length ? `${T.formatNumber(lightPeak, 0)}%` : '—',
          metrics,
          kpis,
          mixItems,
          timeline,
          timelineSegments,
          eventsDisplay,
          framesView,
          heatmapCells,
          riskText: this.buildRiskText(insights),
          updatedAgo: this.data.lastUpdated ? T.formatRelative(this.data.lastUpdated) : '等待同步',
          uptimeText,
          firmwareText,
          reportInterval,
          aiSourceLabel: aiReport ? (aiReport.source === 'ai' ? `AI · ${aiReport.model}` : '本地规则') : '',
          aiFindings: aiReport ? this.decorateFindings(aiReport.findings) : [],
          aiRiskWindows: aiReport ? aiReport.risk_windows || [] : [],
          aiSuggestions: aiReport ? aiReport.suggestions || [] : []
        },
        replayStatics,
        replay,
        frame
      )
    );
  },

  buildReplayStatics(timeline) {
    const replayValues = [];
    const replaySegments = [];
    (timeline || []).forEach((item, index) => {
      replayValues.push(T.numberValue(item.light, 0));
      replaySegments.push({
        index,
        color: T.statusMeta(T.normalizeStatus(item.status)).color,
        opacity: 0.58
      });
    });
    return { replayValues, replaySegments };
  },

  applyReplay() {
    const display = this.computeDisplay(this.data.latest || [], this.data.insights, this.data.aiReport);
    this.setData(Object.assign({ display }, this.computeReplay(display)));
  },

  computeDisplay(latest, insights, aiReport) {
    const replay = this.data.replayActive && insights && insights.timeline && insights.timeline.length > 0;
    let point = null;
    if (replay) {
      const index = T.clamp(this.data.replayIndex, 0, insights.timeline.length - 1);
      point = insights.timeline[index];
    }

    const latestStatus = T.normalizeStatus(
      this.latestText(latest, 'Posture', 'postureStatus', 'unknown')
    );
    const status = point ? T.normalizeStatus(point.status) : latestStatus;
    const meta = T.statusMeta(status);
    const person = point ? !!point.person : this.latestNumber(latest, 'Posture', 'personDetected', 0) === 1;
    const alert =
      point ? !!point.alert : status === 'bad' || status === 'warning' || this.latestNumber(latest, 'Posture', 'alert', 0) === 1;
    const issueFromServer = this.latestNumber(latest, 'Posture', 'issueCount', 0);
    const issue = point
      ? alert
        ? 1
        : 0
      : Math.max(issueFromServer, status === 'bad' ? 2 : status === 'warning' ? 1 : 0);
    const summary = point
      ? `${meta.zh} · 历史回放`
      : this.latestText(latest, 'Posture', 'postureSummary', meta.zh);
    const device = this.findDevice(this.data.selectedDeviceId);
    const lastSeen = point ? point.time : device ? device.last_seen_at : '';
    const light = point ? T.numberValue(point.light, 0) : this.latestNumber(latest, 'Environment', 'light', 0);
    const sound = point
      ? point.sound
        ? 1
        : 0
      : this.latestNumber(latest, 'Environment', 'sound', 0);

    return {
      status,
      statusLabel: meta.label,
      statusZh: meta.zh,
      statusColor: meta.color,
      statusSoft: meta.soft,
      faceClass: meta.faceClass,
      summary,
      person,
      alert,
      issue,
      seq: point ? 0 : this.latestNumber(latest, 'Posture', 'lastSeq', 0),
      lastSeen,
      lastSeenLabel: lastSeen ? T.formatRelative(lastSeen) : '—',
      light,
      sound,
      sedentary: summary.indexOf('久坐') >= 0
    };
  },

  computeReplay(display) {
    const insights = this.data.insights;
    const timeline = insights ? insights.timeline || [] : [];
    const length = timeline.length;
    const active = !!this.data.replayActive;
    const displayIndex = length === 0 ? 0 : active ? T.clamp(this.data.replayIndex, 0, length - 1) : length - 1;
    const point = length > 0 ? timeline[displayIndex] : null;
    const status = active && point ? T.normalizeStatus(point.status) : display.status;
    const meta = T.statusMeta(status);
    const visible = [];
    if (length > 0) {
      const start = Math.max(0, displayIndex - 5);
      const end = Math.min(length, displayIndex + 6);
      for (let i = start; i < end; i += 1) {
        const item = timeline[i];
        const itemMeta = T.statusMeta(T.normalizeStatus(item.status));
        visible.push({
          index: i,
          time: T.formatDateTime(item.time),
          statusZh: itemMeta.zh,
          statusColor: itemMeta.color,
          light: `${T.formatNumber(item.light, 0)}%`
        });
      }
    }

    return {
      replayIndex: active ? displayIndex : this.data.replayIndex,
      replayDisplayIndex: displayIndex,
      replayMax: Math.max(0, length - 1),
      replayTime: point ? T.formatDateTime(point.time) : '—',
      replayStatus: status,
      replayStatusLabel: meta.label,
      replayStatusColor: meta.color,
      replayLight: active && point ? T.numberValue(point.light, 0) : display.light,
      replayPerson: active && point ? !!point.person : display.person,
      replayAlert: active && point ? !!point.alert : display.alert,
      replayVisible: visible
    };
  },

  buildMetrics(latest, display) {
    const rssi = this.latestNumber(latest, 'DeviceStatus', 'rssi', 0);
    const heap = this.latestNumber(latest, 'DeviceStatus', 'freeHeap', 0);
    const uptime = this.latestNumber(latest, 'DeviceStatus', 'uptime', 0);
    return [
      {
        icon: '☀',
        label: '桌面光照',
        value: T.formatNumber(display.light, 0),
        unit: '%',
        hint: display.light >= 60 ? '光照充足' : display.light >= 30 ? '光照适中' : '光照偏暗',
        accent: '#fbbf24',
        live: !this.data.replayActive
      },
      {
        icon: '♪',
        label: '环境声音',
        value: T.formatNumber(display.sound, 0),
        unit: 'LEVEL',
        hint: display.sound === 1 ? '检测到声音' : '环境安静',
        accent: '#a78bfa',
        live: false
      },
      {
        icon: '℃',
        label: '桌面温度',
        value: T.formatNumber(this.latestNumber(latest, 'Environment', 'temperature', 0), 1),
        unit: '°C',
        hint: '传感器离线 · 固定上报 0',
        accent: '#fb7185',
        live: false
      },
      {
        icon: '◌',
        label: '桌面湿度',
        value: T.formatNumber(this.latestNumber(latest, 'Environment', 'humidity', 0), 0),
        unit: '%',
        hint: '传感器离线 · 固定上报 0',
        accent: '#22d3ee',
        live: false
      },
      {
        icon: '▮',
        label: '设备信号',
        value: T.formatNumber(rssi, 0),
        unit: 'dBm',
        hint: 'RSSI',
        accent: '#34d399',
        live: !this.data.replayActive
      },
      {
        icon: '▦',
        label: '设备内存',
        value: T.formatNumber(heap / 1024, 0),
        unit: 'KB',
        hint: `uptime ${this.formatUptime(uptime)}`,
        accent: '#94a3b8',
        live: false
      }
    ];
  },

  buildKpis(insights) {
    if (!insights || !insights.summary) {
      return [
        { label: '今日良好率', value: '—', hint: '等待统计', accent: '#34d399' },
        { label: '今日提醒', value: '—', hint: '累计告警分钟', accent: '#fb7185' },
        { label: '最长告警', value: '—', hint: '连续不良坐姿', accent: '#fbbf24' },
        { label: '平均光照', value: '—', hint: '舒适度', accent: '#22d3ee' }
      ];
    }
    const summary = insights.summary;
    return [
      {
        label: '今日良好率',
        value: T.formatPercent(summary.good_rate),
        hint: `${summary.good} 个良好样本`,
        accent: '#34d399'
      },
      {
        label: '今日提醒',
        value: `${summary.alert_events} 次`,
        hint: `累计 ${summary.alert_minutes} 分钟`,
        accent: '#fb7185'
      },
      {
        label: '最长告警',
        value: `${summary.longest_bad_streak_minutes} 分`,
        hint: '连续不良坐姿',
        accent: '#fbbf24'
      },
      {
        label: '平均光照',
        value: `${T.formatNumber(summary.avg_light, 0)}%`,
        hint: `舒适度 ${T.formatNumber(summary.light_comfort * 100, 0)}%`,
        accent: '#22d3ee'
      }
    ];
  },

  buildMix(insights) {
    if (!insights || !insights.summary) {
      return [];
    }
    const summary = insights.summary;
    const items = [
      { key: 'good', label: '坐姿良好', count: summary.good, color: '#34d399' },
      { key: 'warning', label: '坐姿偏移', count: summary.warning, color: '#fbbf24' },
      { key: 'bad', label: '坐姿异常', count: summary.bad, color: '#fb7185' },
      { key: 'unknown', label: '暂时离开', count: summary.unknown, color: '#94a3b8' }
    ];
    const total = items.reduce((sum, item) => sum + item.count, 0);
    return items.map((item) => ({
      key: item.key,
      label: item.label,
      count: item.count,
      color: item.color,
      percent: total === 0 ? 0 : Math.round((item.count / total) * 100)
    }));
  },

  buildTimelineSegments(timeline) {
    const recent = timeline.length <= 60 ? timeline : timeline.slice(timeline.length - 60);
    return recent.map((item, index) => {
      const meta = T.statusMeta(T.normalizeStatus(item.status));
      return { index, color: meta.color, opacity: 0.55 };
    });
  },

  buildEvents(events) {
    return (events || []).slice(0, 6).map((event) => {
      let title = '设备事件';
      let color = '#22d3ee';
      if (event.resource === 'device.property') {
        title = '坐姿 / 环境属性上报';
        color = '#22d3ee';
      } else if (event.resource === 'device.status') {
        title = `设备状态 · ${event.event || 'update'}`;
        color = '#34d399';
      } else if (event.resource === 'device.message') {
        title = '设备消息';
        color = '#a78bfa';
      }
      const time = event.event_time || event.received_at;
      return {
        id: event.id,
        title,
        color,
        time: T.formatClock(time),
        ago: T.formatRelative(time)
      };
    });
  },

  buildFramesView(frames) {
    return (frames || []).map((frame) => {
      const meta = T.statusMeta(T.normalizeStatus(frame.status));
      return {
        id: frame.id,
        time: T.formatClock(frame.captured_at),
        status: meta.label,
        statusZh: meta.zh,
        color: meta.color,
        selected: frame.id === this.data.selectedFrameId
      };
    });
  },

  buildMessagesView(messages) {
    return (messages || []).map((message) => ({
      id: message.id,
      author: message.author,
      authorName: message.author_name || (message.author === 'child' ? '孩子' : '家长'),
      content: message.content || '',
      ago: T.formatRelative(message.created_at),
      deliveredLabel: message.delivered_at ? '已送达 ESP 屏幕' : '等待 ESP 拉取',
      delivered: !!message.delivered_at
    }));
  },

  onMessageInput(event) {
    this.setData({ messageDraft: event.detail.value });
  },

  onMessageRole(event) {
    const role = event.currentTarget.dataset.role;
    if (role === 'parent' || role === 'child') {
      this.setData({ messageRole: role });
    }
  },

  sendMessage() {
    const draft = `${this.data.messageDraft || ''}`.trim();
    if (!draft || this.data.messageSending || !this.data.selectedDeviceId) {
      return;
    }
    const role = this.data.messageRole;
    const authorName = role === 'parent' ? '妈妈' : '小宇';
    this.setData({ messageSending: true, messageError: '' });
    api
      .sendMessage(this.data.selectedDeviceId, role, authorName, draft)
      .then(() => api.getMessages(this.data.selectedDeviceId, 30))
      .then((messages) => {
        this.setData({
          messages,
          messagesDisplay: this.buildMessagesView(messages),
          messageDraft: '',
          messageSending: false
        });
        wx.showToast({ title: '留言已发送', icon: 'none' });
      })
      .catch((error) => {
        this.setData({
          messageSending: false,
          messageError: error.message || `${error}`
        });
      });
  },

  buildRiskText(insights) {
    if (!insights || !insights.risk_windows || insights.risk_windows.length === 0) {
      return '暂未发现明显的高风险时段';
    }
    const window = insights.risk_windows[0];
    const endHour = (window.hour + 1) % 24;
    return `${T.weekdayName(window.weekday)} ${String(window.hour).padStart(2, '0')}:00–${String(endHour).padStart(2, '0')}:00 · 告警率 ${T.formatPercent(window.alert_rate)}`;
  },

  decorateFindings(findings) {
    return (findings || []).map((finding) => ({
      title: finding.title,
      detail: finding.detail,
      severity: finding.severity,
      color: T.severityColor(finding.severity)
    }));
  },

  computeFrame() {
    const selected = (this.data.frames || []).find((frame) => frame.id === this.data.selectedFrameId);
    if (!selected) {
      return {
        frameStatus: 'unknown',
        frameStatusLabel: '',
        frameStatusZh: '',
        frameStatusColor: '#94a3b8',
        frameSummary: '',
        frameTime: '—',
        framesView: this.buildFramesView(this.data.frames)
      };
    }
    const meta = T.statusMeta(T.normalizeStatus(selected.status));
    return {
      frameStatus: meta.key,
      frameStatusLabel: meta.label,
      frameStatusZh: meta.zh,
      frameStatusColor: meta.color,
      frameSummary: selected.summary || '',
      frameTime: T.formatClock(selected.captured_at),
      framesView: this.buildFramesView(this.data.frames)
    };
  },

  autoSelectFrame(frames) {
    if (!frames || frames.length === 0) {
      this.setData({
        selectedFrameId: 0,
        frameImage: '',
        frameStatus: 'unknown',
        frameStatusLabel: '',
        frameStatusZh: '',
        frameStatusColor: '#94a3b8',
        frameSummary: '',
        frameTime: '—',
        framesView: []
      });
      return;
    }
    const current = frames.find((frame) => frame.id === this.data.selectedFrameId);
    if (current) {
      this.loadFrame(current.id, false);
      return;
    }
    this.loadFrame(frames[0].id, true);
  },

  loadFrame(frameId, force) {
    if (!frameId) {
      return;
    }
    if (!force && frameId === this.data.selectedFrameId && this.data.frameImage) {
      return;
    }
    this.setData({
      selectedFrameId: frameId,
      frameLoading: true,
      frameError: ''
    });
    api
      .downloadFrame(frameId)
      .then((path) => {
        this.setData(
          {
            frameImage: path,
            frameLoading: false,
            frameError: ''
          },
          () => this.setData(this.computeFrame())
        );
      })
      .catch((error) => {
        this.setData({
          frameLoading: false,
          frameError: error.message || `${error}`
        });
      });
  },

  generateReport() {
    if (this.data.aiLoading || !this.data.selectedDeviceId) {
      return;
    }
    this.setData({ aiLoading: true, aiError: '' });
    api
      .generateAIReport(this.data.selectedDeviceId, 24)
      .then((report) => {
        const decorated = Object.assign({}, report, {
          scoreColor: T.scoreColor(report.score),
          levelLabel: T.levelLabel(report.level)
        });
        this.setData(
          {
            aiReport: decorated,
            aiLoading: false,
            aiError: ''
          },
          () => this.applyAll()
        );
      })
      .catch((error) => {
        this.setData({
          aiLoading: false,
          aiError: error.message || `${error}`
        });
      });
  },

  onDemoTap(event) {
    const key = event.currentTarget.dataset.key;
    if (key) {
      this.runDemo(key);
    }
  },

  runDemo(status) {
    if (this.data.demoBusy || !this.data.selectedDeviceId) {
      return;
    }
    this.setData({ demoBusy: true, demoMessage: '' });
    api
      .runDemoScenario(this.data.selectedDeviceId, status)
      .then((result) => {
        const message = `已注入：${result.summary} · ${T.formatClock(result.event_time)}`;
        this.setData({ demoMessage: message });
        wx.showToast({ title: message, icon: 'none', duration: 2200 });
        if (status === 'bad' || status === 'warning') {
          try {
            wx.vibrateShort({ type: 'light' });
          } catch (error) {
            // Haptics are best-effort.
          }
        }
        return this.load(true);
      })
      .catch((error) => {
        this.setData({ demoMessage: error.message || `${error}` });
      })
      .then(() => {
        this.setData({ demoBusy: false });
      });
  },

  switchTab(event) {
    const key = event.currentTarget.dataset.key;
    if (!key || key === this.data.activeTab) {
      return;
    }
    this.setData({ activeTab: key }, () => {
      if (key === 'replay') {
        this.applyReplay();
      }
    });
    try {
      wx.vibrateShort({ type: 'light' });
    } catch (error) {
      // Ignore.
    }
  },

  onRangeChange(event) {
    const range = event.currentTarget.dataset.range;
    if (!range || range === this.data.range) {
      return;
    }
    this.setData({ range }, () => this.load(false));
  },

  onDeviceChange(event) {
    const index = Number(event.detail.value);
    if (Number.isNaN(index) || index === this.data.deviceIndex) {
      return;
    }
    const device = this.data.devices[index];
    if (!device) {
      return;
    }
    this.setData(
      {
        deviceIndex: index,
        selectedDeviceId: device.device_id,
        deviceLabel: this.data.deviceLabels[index],
        aiReport: null,
        aiFindings: [],
        aiRiskWindows: [],
        aiSuggestions: [],
        aiError: '',
        replayActive: false,
        replayPlaying: false,
        replayIndex: 0
      },
      () => {
        this.autoReportDeviceId = '';
        this.load(false);
      }
    );
  },

  onReplaySlider(event) {
    const value = Number(event.detail.value);
    this.setData(
      {
        replayIndex: Number.isNaN(value) ? 0 : value,
        replayActive: true,
        replayPlaying: false
      },
      () => this.applyReplay()
    );
  },

  startReplay() {
    if (!this.data.timeline || this.data.timeline.length === 0) {
      return;
    }
    this.setData({ replayActive: true, replayPlaying: true, replayIndex: 0 }, () =>
      this.applyReplay()
    );
  },

  toggleReplayPlaying() {
    if (!this.data.replayActive) {
      this.startReplay();
      return;
    }
    let index = this.data.replayIndex;
    if (index >= this.data.timeline.length - 1) {
      index = 0;
    }
    this.setData(
      {
        replayIndex: index,
        replayPlaying: !this.data.replayPlaying
      },
      () => this.applyReplay()
    );
  },

  exitReplay() {
    this.setData(
      {
        replayActive: false,
        replayPlaying: false,
        replayIndex: 0
      },
      () => this.applyReplay()
    );
  },

  onReplayPointTap(event) {
    const index = Number(event.currentTarget.dataset.index);
    if (Number.isNaN(index)) {
      return;
    }
    this.setData(
      {
        replayIndex: index,
        replayActive: true,
        replayPlaying: false
      },
      () => this.applyReplay()
    );
  },

  onHeatMetric(event) {
    const key = event.currentTarget.dataset.key;
    if (!key || key === this.data.heatMetric) {
      return;
    }
    this.setData({ heatMetric: key });
  },

  onHeatCellTap(event) {
    const cell = event.detail.cell;
    if (!cell) {
      return;
    }
    wx.showToast({ title: cell.label, icon: 'none', duration: 2400 });
  },

  onSelectFrame(event) {
    const id = Number(event.currentTarget.dataset.id);
    if (!id) {
      return;
    }
    this.loadFrame(id, true);
  },

  previewFrame() {
    if (!this.data.frameImage) {
      return;
    }
    wx.previewImage({
      current: this.data.frameImage,
      urls: [this.data.frameImage]
    });
  },

  onRefresh() {
    this.setData({ refreshing: true });
    this.load(false);
  },

  onAutoRefreshChange(event) {
    this.setData({ autoRefresh: !!event.detail.value });
  },

  onAutoDemoChange(event) {
    this.setData({ autoDemo: !!event.detail.value }, () => this.syncDemoTimer());
  },

  openSettings() {
    const config = api.getConfig();
    this.setData({
      showSettings: true,
      editBaseUrl: config.baseUrl,
      editToken: config.token
    });
  },

  closeSettings() {
    this.setData({ showSettings: false });
  },

  noop() {},

  onBaseInput(event) {
    this.setData({ editBaseUrl: event.detail.value });
  },

  onTokenInput(event) {
    this.setData({ editToken: event.detail.value });
  },

  saveSettings() {
    const config = api.saveConfig(this.data.editBaseUrl, this.data.editToken);
    this.setData(
      {
        showSettings: false,
        apiBaseUrl: config.baseUrl,
        tokenMasked: T.maskToken(config.token),
        editBaseUrl: config.baseUrl,
        editToken: config.token,
        aiReport: null,
        aiFindings: [],
        aiRiskWindows: [],
        aiSuggestions: [],
        aiError: ''
      },
      () => {
        this.autoReportDeviceId = '';
        wx.showToast({ title: '已保存，正在刷新', icon: 'none' });
        this.load(false);
      }
    );
  },

  findDevice(deviceId) {
    return (this.data.devices || []).find((device) => device.device_id === deviceId);
  },

  deviceLabel(device) {
    if (device.device_id.indexOf('hbot') >= 0) {
      return '孩子的书桌';
    }
    if (device.node_id && device.node_id !== device.device_id) {
      return device.node_id;
    }
    return device.device_id;
  },

  latestValue(latest, service, property) {
    const rows = latest || [];
    for (let i = 0; i < rows.length; i += 1) {
      if (rows[i].service_id === service && rows[i].property === property) {
        return rows[i].value;
      }
    }
    return undefined;
  },

  latestNumber(latest, service, property, fallback) {
    return T.numberValue(this.latestValue(latest, service, property), fallback);
  },

  latestText(latest, service, property, fallback) {
    return T.textValue(this.latestValue(latest, service, property), fallback);
  },

  formatUptime(seconds) {
    if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds <= 0) {
      return '—';
    }
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
  }
});
