-- Rainmeter drawing adapter. Configuration changes and drawing are separate:
-- updateMeter/updateGroup apply layout changes; move reuses an existing layout
-- and schedules painting even when every meter has UpdateDivider=-1.
return function(skin)
    local renderer, positions = {}, {}
    local function pixel(value)
        return value < 0 and -math.floor(-value + 0.5) or math.floor(value + 0.5)
    end

    function renderer:move(name, x, y)
        x, y = pixel(x), pixel(y or 0)
        local previous = positions[name]
        if previous and previous.x == x and previous.y == y then return false end
        if not previous then previous = {}; positions[name] = previous end
        previous.x, previous.y = x, y
        -- Unlike Lua SetX/SetY, MoveMeter invalidates an all-static skin.
        -- It also persists coordinates without rebuilding the text layout.
        skin:Bang("!MoveMeter", tostring(x), tostring(y), name)
        return true
    end

    function renderer:setVariable(name, value)
        skin:Bang("!SetVariable", name, tostring(value))
    end

    function renderer:setOption(meter, option, value)
        skin:Bang("!SetOption", meter, option, tostring(value))
    end

    function renderer:updateMeasure(name)
        skin:Bang("!UpdateMeasure", name)
    end

    function renderer:updateMeter(name)
        skin:Bang("!UpdateMeter", name)
    end

    function renderer:updateGroup(name)
        skin:Bang("!UpdateMeterGroup", name)
    end

    function renderer:showMeter(name, visible)
        skin:Bang(visible and "!ShowMeter" or "!HideMeter", name)
    end

    function renderer:showGroup(name, visible)
        skin:Bang(visible and "!ShowMeterGroup" or "!HideMeterGroup", name)
    end

    return renderer
end
