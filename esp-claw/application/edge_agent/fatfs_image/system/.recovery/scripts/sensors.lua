-- Desk Health Assistant sensor front-end: DHT11 on GPIO45, light on GPIO3,
-- sound (digital threshold) on GPIO46.
local gpio=require("gpio")
local delay=require("delay")
local adc=require("adc")

local DHT=45
local DHT_ENABLED=false
local LIGHT=3
local MIC=46

pcall(gpio.set_direction,MIC,"input")

local light_adc
local lok,la=pcall(adc.new,LIGHT,{atten=3,bitwidth=12})
if lok then light_adc=la end

local function dht_once()
  gpio.set_direction(DHT,"output")
  gpio.set_level(DHT,0)
  delay.delay_ms(20)
  gpio.set_level(DHT,1)
  delay.delay_us(30)
  gpio.set_direction(DHT,"input")
  local n=0
  while gpio.get_level(DHT)==1 do n=n+1; if n>3000 then return nil end end
  n=0
  while gpio.get_level(DHT)==0 do n=n+1; if n>3000 then return nil end end
  local b={0,0,0,0,0}
  for i=1,40 do
    n=0
    while gpio.get_level(DHT)==0 do n=n+1; if n>3000 then return nil end end
    local h=0
    while gpio.get_level(DHT)==1 do h=h+1; if h>3000 then return nil end end
    local bit=h>25 and 1 or 0
    local idx=math.floor((i-1)/8)+1
    b[idx]=(b[idx]*2+bit)%256
  end
  if ((b[1]+b[2]+b[3]+b[4])%256)~=b[5] then return nil end
  return b[3],b[1]
end
local function dht_read()
  for _=1,3 do
    local ok,t,h=pcall(dht_once)
    if ok and t then return t,h end
    delay.delay_ms(120)
  end
  return nil
end

local function light_pct()
  if not light_adc then return nil end
  local ok,mv=pcall(light_adc.read_mv,light_adc)
  if not ok or not mv then return nil end
  return math.floor(mv*100/3300)
end

return {
  read=function()
    local temp,hum
    if DHT_ENABLED then temp,hum=dht_read() end
    local light=light_pct()
    local ok,snd=pcall(gpio.get_level,MIC)
    return {
      temp_c=temp,
      humidity=hum,
      light_pct=light,
      sound=(ok and snd==1) and 1 or 0,
    }
  end,
}
