const COLORS = {
  good: '#34d399',
  alert: '#fb7185',
  light: '#fbbf24'
};

const WEEKDAYS = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];

function buildHours() {
  const hours = [];
  for (let i = 0; i < 24; i += 1) {
    hours.push({
      value: i,
      label: i % 3 === 0 ? `${i}` : ''
    });
  }
  return hours;
}

Component({
  properties: {
    cells: {
      type: Array,
      value: []
    },
    metric: {
      type: String,
      value: 'alert'
    }
  },

  data: {
    hours: buildHours(),
    rows: []
  },

  observers: {
    'cells, metric': function onDataChanged(cells, metric) {
      this.buildRows(cells || [], metric || 'alert');
    }
  },

  lifetimes: {
    attached() {
      this.buildRows(this.data.cells || [], this.data.metric || 'alert');
    }
  },

  methods: {
    buildRows(cells, metric) {
      const lookup = {};
      cells.forEach((cell) => {
        lookup[`${cell.weekday}-${cell.hour}`] = cell;
      });
      const baseColor = metric === 'good' ? COLORS.good : metric === 'light' ? COLORS.light : COLORS.alert;
      const rows = [];
      for (let day = 1; day <= 7; day += 1) {
        const rowCells = [];
        for (let hour = 0; hour < 24; hour += 1) {
          const cell = lookup[`${day}-${hour}`];
          if (!cell) {
            rowCells.push({
              day,
              hour,
              color: '#141b28',
              opacity: 1,
              label: `${WEEKDAYS[day - 1]} ${hour}:00 · 无数据`,
              samples: 0
            });
            continue;
          }
          let value = cell.alert_rate;
          if (metric === 'good') {
            value = cell.good_rate;
          } else if (metric === 'light') {
            value = cell.avg_light / 100;
          }
          const clamped = Math.max(0, Math.min(1, value || 0));
          rowCells.push({
            day,
            hour,
            color: baseColor,
            opacity: 0.18 + clamped * 0.82,
            label:
              `${WEEKDAYS[day - 1]} ${hour}:00 · ` +
              `良好率 ${Math.round((cell.good_rate || 0) * 100)}% · ` +
              `告警率 ${Math.round((cell.alert_rate || 0) * 100)}% · ` +
              `光照 ${Math.round(cell.avg_light || 0)}% · ${cell.samples || 0} 样本`,
            samples: cell.samples || 0
          });
        }
        rows.push({
          day,
          dayName: WEEKDAYS[day - 1],
          cells: rowCells
        });
      }
      this.setData({ rows });
    },

    onCellTap(event) {
      const dataset = event.currentTarget.dataset;
      const day = Number(dataset.day);
      const hour = Number(dataset.hour);
      let selected = null;
      this.data.rows.forEach((row) => {
        if (row.day === day) {
          selected = row.cells.find((cell) => cell.hour === hour);
        }
      });
      this.triggerEvent('celltap', {
        day,
        hour,
        cell: selected
      });
    }
  }
});
