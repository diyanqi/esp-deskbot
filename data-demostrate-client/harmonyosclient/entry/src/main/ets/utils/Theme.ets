export interface StatusMeta {
  label: string;
  zh: string;
  color: string;
  soft: string;
}

export interface ColorPalette {
  bg: string;
  bgSoft: string;
  panel: string;
  panelStrong: string;
  line: string;
  lineStrong: string;
  text: string;
  muted: string;
  dim: string;
  cyan: string;
  violet: string;
  emerald: string;
  amber: string;
  rose: string;
  slate: string;
}

export interface StatusMetaMap {
  good: StatusMeta;
  warning: StatusMeta;
  bad: StatusMeta;
  unknown: StatusMeta;
}

export const COLORS: ColorPalette = {
  bg: '#05070C',
  bgSoft: '#080D16',
  panel: '#CC0B111C',
  panelStrong: '#0B111C',
  line: '#1F94A3B8',
  lineStrong: '#3D94A3B8',
  text: '#E6EDF7',
  muted: '#8B9BB4',
  dim: '#5B6B82',
  cyan: '#22D3EE',
  violet: '#A78BFA',
  emerald: '#34D399',
  amber: '#FBBF24',
  rose: '#FB7185',
  slate: '#94A3B8'
};

export const STATUS_META: StatusMetaMap = {
  'good': {
    label: 'GOOD',
    zh: '坐姿良好',
    color: '#34D399',
    soft: '#2634D399'
  },
  'warning': {
    label: 'WARNING',
    zh: '坐姿偏移',
    color: '#FBBF24',
    soft: '#26FBBF24'
  },
  'bad': {
    label: 'ALERT',
    zh: '坐姿异常',
    color: '#FB7185',
    soft: '#26FB7185'
  },
  'unknown': {
    label: 'AWAY',
    zh: '暂时离开',
    color: '#94A3B8',
    soft: '#2694A3B8'
  }
};

export function statusMeta(status: string): StatusMeta {
  if (status === 'good') {
    return STATUS_META.good;
  }
  if (status === 'warning') {
    return STATUS_META.warning;
  }
  if (status === 'bad') {
    return STATUS_META.bad;
  }
  return STATUS_META.unknown;
}

export function scoreColor(score: number): string {
  if (score >= 85) {
    return COLORS.emerald;
  }
  if (score >= 70) {
    return COLORS.cyan;
  }
  if (score >= 55) {
    return COLORS.amber;
  }
  return COLORS.rose;
}

export function levelLabel(level: string): string {
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

export function severityColor(severity: string): string {
  if (severity === 'alert') {
    return COLORS.rose;
  }
  if (severity === 'warn') {
    return COLORS.amber;
  }
  return COLORS.emerald;
}

export function weekdayName(weekday: number): string {
  const names: string[] = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
  if (weekday < 1 || weekday > 7) {
    return '未知';
  }
  return names[weekday - 1];
}

function pad2(value: number): string {
  return value < 10 ? `0${value}` : `${value}`;
}

export function formatClock(iso: string): string {
  if (iso.length === 0) {
    return '—';
  }
  const date: Date = new Date(iso);
  const time: number = date.getTime();
  if (Number.isNaN(time)) {
    return '—';
  }
  return `${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}

export function formatDateTime(iso: string): string {
  if (iso.length === 0) {
    return '—';
  }
  const date: Date = new Date(iso);
  const time: number = date.getTime();
  if (Number.isNaN(time)) {
    return '—';
  }
  return `${date.getMonth() + 1}/${date.getDate()} ${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}

export function formatRelative(iso: string): string {
  if (iso.length === 0) {
    return '—';
  }
  const time: number = new Date(iso).getTime();
  if (Number.isNaN(time)) {
    return '—';
  }
  const seconds: number = Math.round((time - Date.now()) / 1000);
  const absolute: number = Math.abs(seconds);
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

export function formatNumber(value: number, digits: number = 0): string {
  if (!Number.isFinite(value)) {
    return '—';
  }
  return value.toFixed(digits);
}

export function formatPercent(rate: number): string {
  return `${Math.round(rate * 100)}%`;
}

export function textValue(value: Object | null | undefined, fallback: string = ''): string {
  if (value === undefined || value === null) {
    return fallback;
  }
  return `${value}`;
}

export function numberValue(value: Object | null | undefined, fallback: number = 0): number {
  if (value === undefined || value === null) {
    return fallback;
  }
  if (typeof value === 'number') {
    return value as number;
  }
  const parsed: number = Number(`${value}`);
  return Number.isNaN(parsed) ? fallback : parsed;
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
