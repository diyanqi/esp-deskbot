function hexToRgba(hex, alpha) {
  const value = `${hex || '#22d3ee'}`.replace('#', '');
  const normalized = value.length === 3
    ? value.split('').map((char) => `${char}${char}`).join('')
    : value;
  const red = parseInt(normalized.substring(0, 2), 16);
  const green = parseInt(normalized.substring(2, 4), 16);
  const blue = parseInt(normalized.substring(4, 6), 16);
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
}

Component({
  properties: {
    points: {
      type: Array,
      value: []
    },
    color: {
      type: String,
      value: '#22d3ee'
    },
    height: {
      type: Number,
      value: 120
    },
    fill: {
      type: Boolean,
      value: true
    }
  },

  data: {},

  lifetimes: {
    ready() {
      this.initCanvas();
    },
    detached() {
      this.ctx = null;
      this.canvas = null;
    }
  },

  observers: {
    'points, color, height': function onDataChanged() {
      this.scheduleDraw();
    }
  },

  methods: {
    initCanvas() {
      this.createSelectorQuery()
        .select('#chart')
        .fields({ node: true, size: true })
        .exec((result) => {
          const target = result && result[0];
          if (!target || !target.node || !target.width || !target.height) {
            if (!this._retry) {
              this._retry = 0;
            }
            if (this._retry < 8) {
              this._retry += 1;
              setTimeout(() => this.initCanvas(), 160);
            }
            return;
          }
          const dpr = (wx.getWindowInfo ? wx.getWindowInfo().pixelRatio : wx.getSystemInfoSync().pixelRatio) || 2;
          const canvas = target.node;
          canvas.width = target.width * dpr;
          canvas.height = target.height * dpr;
          const ctx = canvas.getContext('2d');
          ctx.scale(dpr, dpr);
          this.canvas = canvas;
          this.ctx = ctx;
          this.width = target.width;
          this.heightPx = target.height;
          this.draw();
        });
    },

    scheduleDraw() {
      if (!this.ctx) {
        return;
      }
      if (this._drawScheduled) {
        return;
      }
      this._drawScheduled = true;
      wx.nextTick(() => {
        this._drawScheduled = false;
        this.draw();
      });
    },

    draw() {
      const ctx = this.ctx;
      if (!ctx || !this.width || !this.heightPx) {
        return;
      }
      const width = this.width;
      const height = this.heightPx;
      const points = (this.data.points || []).filter((value) => typeof value === 'number' && Number.isFinite(value));
      const accent = this.data.color || '#22d3ee';
      const left = 2;
      const right = width - 2;
      const top = 8;
      const bottom = height - 8;

      ctx.clearRect(0, 0, width, height);

      ctx.strokeStyle = 'rgba(148, 163, 184, 0.10)';
      ctx.lineWidth = 1;
      for (let i = 1; i <= 3; i += 1) {
        const y = top + ((bottom - top) * i) / 4;
        ctx.beginPath();
        ctx.moveTo(left, y);
        ctx.lineTo(right, y);
        ctx.stroke();
      }

      if (points.length < 2) {
        return;
      }

      let min = points[0];
      let max = points[0];
      for (let i = 1; i < points.length; i += 1) {
        if (points[i] < min) {
          min = points[i];
        }
        if (points[i] > max) {
          max = points[i];
        }
      }
      const span = max - min === 0 ? 1 : max - min;
      const positions = points.map((value, index) => {
        const x = left + ((right - left) * index) / (points.length - 1);
        const y = bottom - ((value - min) / span) * (bottom - top);
        return { x, y };
      });

      if (this.data.fill !== false) {
        const gradient = ctx.createLinearGradient(0, top, 0, bottom);
        gradient.addColorStop(0, hexToRgba(accent, 0.24));
        gradient.addColorStop(1, hexToRgba(accent, 0));
        ctx.beginPath();
        ctx.moveTo(positions[0].x, bottom);
        positions.forEach((point) => {
          ctx.lineTo(point.x, point.y);
        });
        ctx.lineTo(positions[positions.length - 1].x, bottom);
        ctx.closePath();
        ctx.fillStyle = gradient;
        ctx.fill();
      }

      ctx.beginPath();
      positions.forEach((point, index) => {
        if (index === 0) {
          ctx.moveTo(point.x, point.y);
        } else {
          ctx.lineTo(point.x, point.y);
        }
      });
      ctx.strokeStyle = accent;
      ctx.lineWidth = 2;
      ctx.lineJoin = 'round';
      ctx.lineCap = 'round';
      ctx.stroke();

      const last = positions[positions.length - 1];
      ctx.beginPath();
      ctx.arc(last.x, last.y, 2.4, 0, Math.PI * 2);
      ctx.fillStyle = accent;
      ctx.fill();
    }
  }
});
