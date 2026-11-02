const DEFAULT_BASE_URL = 'http://10.196.144.134:11294';
const DEFAULT_TOKEN = 'test-api-token';
const CONFIG_KEY = 'growth_guardian_api_config';

let currentConfig = null;

function getConfig() {
  if (currentConfig) {
    return currentConfig;
  }
  let stored = null;
  try {
    stored = wx.getStorageSync(CONFIG_KEY);
  } catch (error) {
    stored = null;
  }
  currentConfig = {
    baseUrl: stored && stored.baseUrl ? stored.baseUrl : DEFAULT_BASE_URL,
    token: stored && typeof stored.token === 'string' ? stored.token : DEFAULT_TOKEN
  };
  return currentConfig;
}

function saveConfig(baseUrl, token) {
  const normalizedBase = `${baseUrl || DEFAULT_BASE_URL}`.trim().replace(/\/+$/, '');
  currentConfig = {
    baseUrl: normalizedBase || DEFAULT_BASE_URL,
    token: `${token || ''}`.trim()
  };
  try {
    wx.setStorageSync(CONFIG_KEY, currentConfig);
  } catch (error) {
    // The in-memory config still works for this session.
  }
  return currentConfig;
}

function authHeader() {
  const config = getConfig();
  const header = {
    Accept: 'application/json',
    'Content-Type': 'application/json'
  };
  if (config.token) {
    header.Authorization = `Bearer ${config.token}`;
  }
  return header;
}

function urlOf(path) {
  const config = getConfig();
  return `${config.baseUrl.replace(/\/+$/, '')}${path}`;
}

function errorMessage(body, statusCode) {
  if (body && body.error && body.error.message) {
    return body.error.message;
  }
  if (statusCode === 401) {
    return 'API Token 无效，请在设置里重新填写';
  }
  return `HTTP ${statusCode}`;
}

function request(path, options) {
  const opts = options || {};
  return new Promise((resolve, reject) => {
    wx.request({
      url: urlOf(path),
      method: opts.method || 'GET',
      data: opts.data,
      header: authHeader(),
      timeout: opts.timeout || 20000,
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(res.data);
          return;
        }
        reject(new Error(errorMessage(res.data, res.statusCode)));
      },
      fail: (reason) => {
        reject(new Error(`无法连接数据服务：${reason.errMsg || reason}`));
      }
    });
  });
}

function getDevices() {
  return request('/api/v1/devices?limit=100').then((body) => body.data || []);
}

function getLatest(deviceId) {
  return request(`/api/v1/devices/${encodeURIComponent(deviceId)}/latest`).then(
    (body) => body.data || []
  );
}

function getSeries(deviceId, serviceId, property, from, to, bucket) {
  const query =
    `service_id=${encodeURIComponent(serviceId)}` +
    `&property=${encodeURIComponent(property)}` +
    `&from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}` +
    `&bucket=${encodeURIComponent(bucket)}`;
  return request(`/api/v1/devices/${encodeURIComponent(deviceId)}/series?${query}`).then(
    (body) => body.data || []
  );
}

function getTelemetry(deviceId, serviceId, property, from, to, limit) {
  const query =
    `service_id=${encodeURIComponent(serviceId)}` +
    `&property=${encodeURIComponent(property)}` +
    `&from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}` +
    `&limit=${limit}`;
  return request(`/api/v1/devices/${encodeURIComponent(deviceId)}/telemetry?${query}`).then(
    (body) => body.data || []
  );
}

function getEvents(deviceId, limit) {
  return request(
    `/api/v1/events?device_id=${encodeURIComponent(deviceId)}&limit=${limit}`
  ).then((body) => body.data || []);
}

function getInsights(deviceId, hours) {
  return request(
    `/api/v1/insights?device_id=${encodeURIComponent(deviceId)}&hours=${hours}`
  ).then((body) => body.data);
}

function generateAIReport(deviceId, hours) {
  return request('/api/v1/ai/report', {
    method: 'POST',
    data: { device_id: deviceId, hours },
    timeout: 60000
  }).then((body) => body.data);
}

function listFrames(deviceId, limit) {
  return request(
    `/api/v1/frames?device_id=${encodeURIComponent(deviceId)}&limit=${limit}`
  ).then((body) => body.data || []);
}

function getMessages(deviceId, limit) {
  return request(
    `/api/v1/messages?device_id=${encodeURIComponent(deviceId)}&limit=${limit}`
  ).then((body) => body.data || []);
}

function sendMessage(deviceId, author, authorName, content) {
  return request('/api/v1/messages', {
    method: 'POST',
    data: {
      device_id: deviceId,
      author,
      author_name: authorName,
      content
    },
    timeout: 20000
  }).then((body) => body.data);
}

function downloadFrame(frameId) {
  return new Promise((resolve, reject) => {
    if (!frameId) {
      reject(new Error('没有可展示的标注画面'));
      return;
    }
    const config = getConfig();
    const header = {};
    if (config.token) {
      header.Authorization = `Bearer ${config.token}`;
    }
    wx.downloadFile({
      url: urlOf(`/api/v1/frames/${frameId}/image`),
      header,
      timeout: 20000,
      success: (res) => {
        if (res.statusCode === 200) {
          resolve(res.tempFilePath);
        } else {
          reject(new Error(`标注画面加载失败（HTTP ${res.statusCode}）`));
        }
      },
      fail: (reason) => {
        reject(new Error(`标注画面加载失败：${reason.errMsg || reason}`));
      }
    });
  });
}

function runDemoScenario(deviceId, status) {
  return request('/api/v1/demo/scenario', {
    method: 'POST',
    data: {
      device_id: deviceId,
      status,
      light: status === 'away' ? 25 : 60,
      sound: 0
    },
    timeout: 20000
  }).then((body) => body.data);
}

const RANGE_HOURS = {
  '1H': 1,
  '6H': 6,
  '24H': 24,
  '7D': 168
};

function rangeParams(rangeKey) {
  const hours = RANGE_HOURS[rangeKey] || 24;
  const to = new Date();
  const from = new Date(to.getTime() - hours * 3600 * 1000);
  return {
    from: from.toISOString(),
    to: to.toISOString(),
    bucket: hours <= 6 ? 'minute' : 'hour',
    hours
  };
}

module.exports = {
  DEFAULT_BASE_URL,
  DEFAULT_TOKEN,
  getConfig,
  saveConfig,
  getDevices,
  getLatest,
  getSeries,
  getTelemetry,
  getEvents,
  getInsights,
  generateAIReport,
  listFrames,
  getMessages,
  sendMessage,
  downloadFrame,
  runDemoScenario,
  rangeParams
};
