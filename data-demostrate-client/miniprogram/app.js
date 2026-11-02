// app.js
App({
  globalData: {
    appName: '成长守护',
    apiBaseUrl: '',
    apiToken: ''
  },

  onLaunch() {
    try {
      if (wx.setBackgroundColor) {
        wx.setBackgroundColor({
          backgroundColor: '#05070c',
          backgroundColorTop: '#05070c',
          backgroundColorBottom: '#05070c'
        });
      }
    } catch (error) {
      // Older base libraries may not support setBackgroundColor.
    }
  }
});
