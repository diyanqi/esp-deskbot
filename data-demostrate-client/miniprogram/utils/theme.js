const STATUS_META = {
  good: {
    key: 'good',
    label: 'GOOD',
    zh: '坐姿良好',
    color: '#34d399',
    soft: 'rgba(52, 211, 153, 0.13)',
    faceClass: 'face-good'
  },
  warning: {
    key: 'warning',
    label: 'WARNING',
    zh: '坐姿偏移',
    color: '#fbbf24',
    soft: 'rgba(251, 191, 36, 0.13)',
    faceClass: 'face-warning'
  },
  bad: {
    key: 'bad',
    label: 'ALERT',
    zh: '坐姿异常',
    color: '#fb7185',
    soft: 'rgba(251, 113, 133, 0.13)',
    faceClass: 'face-bad'
  },
  unknown: {
    key: 'unknown',
    label: 'AWAY',
    zh: '暂时离开',
    color: '#94a3b8',
    soft: 'rgba(148, 163, 184, 0.12)',
    faceClass: 'face-unknown'
  }
};

function statusMeta(status) {
  return STATUS_META[status] || STATUS_META.unknown;
}

function scoreColor(score) {
  if (score >= 85) {
    return '#34d399';
  }
  if (score >= 70) {
    return '#22d3ee';
  }
  if (score >= 55) {
    return '#fbbf24';
  }
  return '#fb7185';
}

function levelLabel(level) {
  if (level === 'excellent') {
    return '优秀';
  }
  if (level === 'good') {
    return '良好';
  }
  if (level === 'attention') {
    return '需关注';
  }
  return '需干预';
}

function severityColor(severity) {
  if (severity === 'alert') {
    return '#fb7185';
  }
  if (severity === 'warn') {
    return '#fbbf24';
  }
  return '#34d399';
}

function weekdayName(weekday) {
  const names = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
  return names[weekday - 1] || '未知';
}

function pad2(value) {
  return value < 10 ? `0${value}` : `${value}`;
}

function formatClock(iso) {
  if (!iso) {
    return '—';
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return `${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}

function formatDateTime(iso) {
  if (!iso) {
    return '—';
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return `${date.getMonth() + 1}/${date.getDate()} ${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}

function formatRelative(iso) {
  if (!iso) {
    return '—';
  }
  const time = new Date(iso).getTime();
  if (Number.isNaN(time)) {
    return '—';
  }
  const diffSeconds = Math.round((time - Date.now()) / 1000);
  const absolute = Math.abs(diffSeconds);
  if (absolute < 60) {
    return '刚刚';
  }
  if (absolute < 3600) {
    return `${Math.round(absolute / 60)} 分钟前`;
  }
  if (absolute < 86400) {
    return `${Math.round(absolute / 3600)} 小时前`;
  }
  return formatDateTime(iso);
}

function formatNumber(value, digits) {
  const places = typeof digits === 'number' ? digits : 0;
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return '—';
  }
  return value.toFixed(places);
}

function formatPercent(rate) {
  if (typeof rate !== 'number' || !Number.isFinite(rate)) {
    return '—';
  }
  return `${Math.round(rate * 100)}%`;
}

function normalizeStatus(value) {
  if (value === 'good' || value === 'warning' || value === 'bad') {
    return value;
  }
  return 'unknown';
}

function numberValue(value, fallback) {
  const base = typeof fallback === 'number' ? fallback : 0;
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (value === null || value === undefined) {
    return base;
  }
  const parsed = Number(value);
  return Number.isNaN(parsed) ? base : parsed;
}

function textValue(value, fallback) {
  if (value === null || value === undefined) {
    return typeof fallback === 'string' ? fallback : '';
  }
  return `${value}`;
}

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function maskToken(token) {
  if (!token) {
    return '未设置';
  }
  if (token.length <= 6) {
    return '••••••';
  }
  return `${token.substring(0, 3)}••••${token.substring(token.length - 3)}`;
}

module.exports = {
  STATUS_META,
  statusMeta,
  scoreColor,
  levelLabel,
  severityColor,
  weekdayName,
  formatClock,
  formatDateTime,
  formatRelative,
  formatNumber,
  formatPercent,
  normalizeStatus,
  numberValue,
  textValue,
  clamp,
  maskToken
};
