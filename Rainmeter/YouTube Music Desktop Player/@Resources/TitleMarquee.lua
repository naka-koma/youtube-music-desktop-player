-- Shared single-line title marquee. Text is measured at its natural width and
-- clipped by a fixed container, so scrolling never changes the skin's size.
local meterName, sourceName, containerName
local lastText, lastWidth, lastViewport, lastTick
local offset, elapsed, phase = 0, 0, "start"

function Initialize()
    lastTick = os.clock()
    meterName = SELF:GetOption("TextMeter")
    sourceName = SELF:GetOption("SourceMeasure")
    containerName = SELF:GetOption("ViewportMeter")
end

local function position(value)
    SKIN:Bang("!SetOption", meterName, "X", tostring(-value))
    SKIN:Bang("!UpdateMeter", meterName)
    SKIN:Bang("!Redraw")
end

local function reset()
    offset, elapsed, phase = 0, 0, "start"
    position(0)
end

function Update()
    -- Windows' clock is elapsed wall time. Keep the same speed when Rainmeter
    -- reduces updates for a hidden skin, and avoid a jump after a long sleep.
    local now = os.clock()
    local step = math.max(0, math.min(0.25, now - (lastTick or now)))
    lastTick = now
    local meter, viewport = SKIN:GetMeter(meterName), SKIN:GetMeter(containerName)
    local source = SKIN:GetMeasure(sourceName)
    if not meter or not viewport or not source then return 0 end
    local text = source:GetStringValue() or ""
    if text ~= lastText then
        lastText, lastWidth = text, nil
        reset()
        -- Let Rainmeter render the new text before reading its natural width.
        return 0
    end
    local width, available = meter:GetW(), viewport:GetW()
    if width ~= lastWidth or available ~= lastViewport then
        lastWidth, lastViewport = width, available
        reset()
        return 0
    end
    local overflow = width - available
    if available <= 0 or overflow <= 1 then return 0 end
    local pause = math.max(0, SELF:GetNumberOption("PauseSeconds", 1.8))
    elapsed = elapsed + step
    if phase == "start" then
        if elapsed >= pause then phase, elapsed = "scroll", 0 end
    elseif phase == "scroll" then
        local speed = math.max(1, SELF:GetNumberOption("ScrollSpeed", 30))
        offset = math.min(overflow, offset + speed * step)
        position(offset)
        if offset >= overflow then phase, elapsed = "end", 0 end
    elseif elapsed >= pause then
        reset()
    end
    return offset
end
