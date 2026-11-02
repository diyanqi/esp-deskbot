import { common } from '@kit.AbilityKit';
import { preferences } from '@kit.ArkData';
import { http } from '@kit.NetworkKit';
import { image } from '@kit.ImageKit';

import {
  AIReport,
  AIReportResponse,
  DemoResponse,
  DemoResult,
  Device,
  EventRow,
  FramesResponse,
  Insights,
  InsightsResponse,
  LatestResponse,
  ListResponse,
  Message,
  MessagesResponse,
  MessageResponse,
  PostureFrame,
  RangeParams,
  SeriesPoint,
  SeriesResponse,
  TelemetryRow
} from '../model/DashboardModels';

export const SETTINGS_STORE: string = 'growth_guardian_settings';
export const KEY_BASE_URL: string = 'apiBaseUrl';
export const KEY_API_TOKEN: string = 'apiToken';

/** 开发机地址：真机与 Mac 在同一网段时使用。可在设置抽屉里修改。 */
export const DEFAULT_BASE_URL: string = 'http://10.196.144.134:11294';
export const DEFAULT_API_TOKEN: string = 'test-api-token';

interface APIErrorEnvelope {
  error?: APIErrorBody;
}

interface APIErrorBody {
  message?: string;
  code?: string;
}

interface DeviceResponse {
  data: Device[];
}

interface FramesResponseAlias {
  data: PostureFrame[];
}

interface DemoEnvelope {
  data: DemoResult;
}

interface AIReportRequest {
  device_id: string;
  hours: number;
}

interface DemoScenarioRequest {
  device_id: string;
  status: string;
  light: number;
  sound: number;
}

interface CreateMessageRequest {
  device_id: string;
  author: string;
  author_name: string;
  content: string;
}

function appBaseUrl(): string {
  const value: string | undefined = AppStorage.get<string>(KEY_BASE_URL);
  return value === undefined || value.length === 0 ? DEFAULT_BASE_URL : value;
}

function appToken(): string {
  const value: string | undefined = AppStorage.get<string>(KEY_API_TOKEN);
  return value === undefined ? DEFAULT_API_TOKEN : value;
}

function endpoint(path: string): string {
  const base: string = appBaseUrl().replace(/\/+$/, '');
  return `${base}${path}`;
}

function authHeader(): Record<string, string> {
  const headers: Record<string, string> = {
    'Accept': 'application/json',
    'Content-Type': 'application/json'
  };
  const token: string = appToken();
  if (token.length > 0) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

function errorMessage(text: string, code: number): string {
  try {
    const envelope: APIErrorEnvelope = JSON.parse(text) as APIErrorEnvelope;
    const body: APIErrorBody | undefined = envelope.error;
    if (body !== undefined && body.message !== undefined) {
      return body.message;
    }
  } catch (error) {
    // The response is not a JSON error envelope.
  }
  return code === 401 ? 'API Token 无效，请在设置里重新填写' : `HTTP ${code}`;
}

async function requestText(path: string, method: http.RequestMethod, body: string | undefined,
  timeout: number): Promise<string> {
  const request: http.HttpRequest = http.createHttp();
  const options: http.HttpRequestOptions = {
    method: method,
    header: authHeader(),
    expectDataType: http.HttpDataType.STRING,
    connectTimeout: 6000,
    readTimeout: timeout
  };
  if (body !== undefined) {
    options.extraData = body;
  }
  try {
    const response: http.HttpResponse = await request.request(endpoint(path), options);
    const text: string = `${response.result}`;
    if (response.responseCode < 200 || response.responseCode >= 300) {
      throw new Error(errorMessage(text, response.responseCode));
    }
    return text;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error(`无法连接数据服务：${error}`);
  } finally {
    request.destroy();
  }
}

async function getJSON<T>(path: string, timeout: number = 20000): Promise<T> {
  const text: string = await requestText(path, http.RequestMethod.GET, undefined, timeout);
  return JSON.parse(text) as T;
}

async function postJSON<T>(path: string, body: Object, timeout: number = 60000): Promise<T> {
  const text: string = await requestText(path, http.RequestMethod.POST, JSON.stringify(body), timeout);
  return JSON.parse(text) as T;
}

export function initSettings(context: common.Context): void {
  try {
    const store: preferences.Preferences =
      preferences.getPreferencesSync(context, { name: SETTINGS_STORE });
    const base: string = store.getSync(KEY_BASE_URL, DEFAULT_BASE_URL) as string;
    const token: string = store.getSync(KEY_API_TOKEN, DEFAULT_API_TOKEN) as string;
    AppStorage.setOrCreate(KEY_BASE_URL, base.length > 0 ? base : DEFAULT_BASE_URL);
    AppStorage.setOrCreate(KEY_API_TOKEN, token);
  } catch (error) {
    AppStorage.setOrCreate(KEY_BASE_URL, DEFAULT_BASE_URL);
    AppStorage.setOrCreate(KEY_API_TOKEN, DEFAULT_API_TOKEN);
  }
}

export function saveSettings(context: common.Context, baseUrl: string, token: string): void {
  const base: string = baseUrl.trim().replace(/\/+$/, '');
  const normalizedBase: string = base.length > 0 ? base : DEFAULT_BASE_URL;
  AppStorage.setOrCreate(KEY_BASE_URL, normalizedBase);
  AppStorage.setOrCreate(KEY_API_TOKEN, token.trim());
  try {
    const store: preferences.Preferences =
      preferences.getPreferencesSync(context, { name: SETTINGS_STORE });
    store.putSync(KEY_BASE_URL, normalizedBase);
    store.putSync(KEY_API_TOKEN, token.trim());
    store.flush();
  } catch (error) {
    // AppStorage still carries the new values for this session.
  }
}

export function currentBaseUrl(): string {
  return appBaseUrl();
}

export function currentToken(): string {
  return appToken();
}

export async function listDevices(): Promise<Device[]> {
  const response: DeviceResponse =
    await getJSON<DeviceResponse>('/api/v1/devices?limit=100');
  return response.data === undefined ? [] : response.data;
}

export async function getLatest(deviceId: string): Promise<TelemetryRow[]> {
  const response: LatestResponse = await getJSON<LatestResponse>(
    `/api/v1/devices/${encodeURIComponent(deviceId)}/latest`);
  return response.data === undefined ? [] : response.data;
}

export async function getSeries(deviceId: string, serviceId: string, property: string,
  from: string, to: string, bucket: string): Promise<SeriesPoint[]> {
  const path: string = `/api/v1/devices/${encodeURIComponent(deviceId)}/series?` +
    `service_id=${encodeURIComponent(serviceId)}&property=${encodeURIComponent(property)}` +
    `&from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&bucket=${encodeURIComponent(bucket)}`;
  const response: SeriesResponse = await getJSON<SeriesResponse>(path);
  return response.data === undefined ? [] : response.data;
}

export async function getTelemetry(deviceId: string, serviceId: string, property: string,
  from: string, to: string, limit: number): Promise<TelemetryRow[]> {
  const path: string = `/api/v1/devices/${encodeURIComponent(deviceId)}/telemetry?` +
    `service_id=${encodeURIComponent(serviceId)}&property=${encodeURIComponent(property)}` +
    `&from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&limit=${limit}`;
  const response: ListResponse<TelemetryRow> = await getJSON<ListResponse<TelemetryRow>>(path);
  return response.data === undefined ? [] : response.data;
}

export async function getEvents(deviceId: string, limit: number): Promise<EventRow[]> {
  const path: string = `/api/v1/events?device_id=${encodeURIComponent(deviceId)}&limit=${limit}`;
  const response: ListResponse<EventRow> = await getJSON<ListResponse<EventRow>>(path);
  return response.data === undefined ? [] : response.data;
}

export async function getInsights(deviceId: string, hours: number): Promise<Insights> {
  const response: InsightsResponse = await getJSON<InsightsResponse>(
    `/api/v1/insights?device_id=${encodeURIComponent(deviceId)}&hours=${hours}`);
  return response.data;
}

export async function generateAIReport(deviceId: string, hours: number): Promise<AIReport> {
  const body: AIReportRequest = {
    device_id: deviceId,
    hours: hours
  };
  const response: AIReportResponse =
    await postJSON<AIReportResponse>('/api/v1/ai/report', body, 60000);
  return response.data;
}

export async function listFrames(deviceId: string, limit: number): Promise<PostureFrame[]> {
  const response: FramesResponse = await getJSON<FramesResponse>(
    `/api/v1/frames?device_id=${encodeURIComponent(deviceId)}&limit=${limit}`);
  return response.data === undefined ? [] : response.data;
}

export async function listMessages(deviceId: string, limit: number): Promise<Message[]> {
  const response: MessagesResponse = await getJSON<MessagesResponse>(
    `/api/v1/messages?device_id=${encodeURIComponent(deviceId)}&limit=${limit}`);
  return response.data === undefined ? [] : response.data;
}

export async function sendMessage(deviceId: string, author: string,
  authorName: string, content: string): Promise<Message> {
  const body: CreateMessageRequest = {
    device_id: deviceId,
    author: author,
    author_name: authorName,
    content: content
  };
  const response: MessageResponse =
    await postJSON<MessageResponse>('/api/v1/messages', body, 20000);
  return response.data;
}

export async function getFramePixelMap(frameId: number): Promise<image.PixelMap> {
  const request: http.HttpRequest = http.createHttp();
  const headers: Record<string, string> = {
    'Accept': 'image/*'
  };
  const token: string = appToken();
  if (token.length > 0) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const options: http.HttpRequestOptions = {
    method: http.RequestMethod.GET,
    header: headers,
    expectDataType: http.HttpDataType.ARRAY_BUFFER,
    connectTimeout: 6000,
    readTimeout: 20000
  };
  try {
    const response: http.HttpResponse =
      await request.request(endpoint(`/api/v1/frames/${frameId}/image`), options);
    if (response.responseCode !== 200) {
      throw new Error(`标注画面加载失败（HTTP ${response.responseCode}）`);
    }
    const bytes: ArrayBuffer = response.result as ArrayBuffer;
    if (bytes.byteLength === 0) {
      throw new Error('标注画面为空');
    }
    const source: image.ImageSource = image.createImageSource(bytes);
    const pixelMap: image.PixelMap = await source.createPixelMap();
    source.release();
    return pixelMap;
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error(`标注画面加载失败：${error}`);
  } finally {
    request.destroy();
  }
}

export async function runDemoScenario(deviceId: string, status: string): Promise<DemoResult> {
  const body: DemoScenarioRequest = {
    device_id: deviceId,
    status: status,
    light: status === 'away' ? 25 : 60,
    sound: 0
  };
  const response: DemoEnvelope =
    await postJSON<DemoEnvelope>('/api/v1/demo/scenario', body, 20000);
  return response.data;
}

const RANGE_MAP: Record<string, number> = {
  '1H': 1,
  '6H': 6,
  '24H': 24,
  '7D': 168
};

export function rangeParams(rangeKey: string): RangeParams {
  const hours: number = RANGE_MAP[rangeKey] === undefined ? 24 : RANGE_MAP[rangeKey];
  const to: Date = new Date();
  const from: Date = new Date(to.getTime() - hours * 3600 * 1000);
  return {
    from: from.toISOString(),
    to: to.toISOString(),
    bucket: hours <= 6 ? 'minute' : 'hour',
    hours: hours
  };
}
