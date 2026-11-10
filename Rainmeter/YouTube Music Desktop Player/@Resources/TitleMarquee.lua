-- Rainmeter adapter shared by compact and fullscreen titles.
local meterName, sourceName, containerName, renderer, marquee
local lastText, lastWidth, lastViewport, lastTick

function Initialize()
    local components = SKIN:GetVariable("@") .. "Components/"
    renderer = dofile(components .. "Renderer.lua")(SKIN)
    marquee = dofile(components .. "Marquee.lua")()
    lastTick = os.clock()
    meterName = SELF:GetOption("TextMeter")
    sourceName = SELF:GetOption("SourceMeasure")
    containerName = SELF:GetOption("ViewportMeter")
end

local function reset()
    marquee:reset()
    renderer:move(meterName, 0, 0)
end

function Update()
    local now = os.clock()
    local seconds = now - (lastTick or now)
    lastTick = now
    local meter, viewport = SKIN:GetMeter(meterName), SKIN:GetMeter(containerName)
    local source = SKIN:GetMeasure(sourceName)
    if not meter or not viewport or not source then return 0 end
    local text = source:GetStringValue() or ""
    if text ~= lastText then
        lastText, lastWidth = text, nil
        renderer:updateMeter(meterName)
        reset()
        -- Measure the new text after Rainmeter has applied its layout update.
        return 0
    end
    local width, available = meter:GetW(), viewport:GetW()
    if width ~= lastWidth or available ~= lastViewport then
        lastWidth, lastViewport = width, available
        reset()
        return 0
    end
    local enabled = available > 0 and SKIN:GetVariable("RenderMarquee", "1") ~= "0"
    local offset = marquee:step(seconds, width - available,
        SELF:GetNumberOption("ScrollSpeed", 30), SELF:GetNumberOption("PauseSeconds", 1.8), enabled)
    renderer:move(meterName, -offset, 0)
    return offset
end
