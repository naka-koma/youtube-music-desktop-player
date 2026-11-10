local profiles = {
    {id="power", label="Power Saver", interval=100, marquee=0},
    {id="balanced", label="Balanced", interval=66, marquee=1},
    {id="smooth", label="Smooth", interval=33, marquee=1}
}

local function rate(milliseconds, interval)
    return math.max(1, math.floor(milliseconds / interval + 0.5))
end

function Initialize()
    local selected = SKIN:GetVariable("RenderMode", "balanced")
    local offset = tonumber(SKIN:GetVariable("PerformanceMenuOffset", "0")) or 0
    for i, profile in ipairs(profiles) do
        local index = offset + i
        local suffix = index == 1 and "" or tostring(index)
        local label = profile.label .. (selected == profile.id and " (selected)" or "")
        SKIN:Bang("!SetOption", "Rainmeter", "ContextTitle" .. suffix, label)
    end
end

function Select(id)
    local chosen
    for _, profile in ipairs(profiles) do
        if profile.id == id then chosen = profile; break end
    end
    if not chosen or SKIN:GetVariable("RenderMode", "balanced") == id then return end
    local interval = chosen.interval
    local values = {
        {"RenderMode", chosen.id}, {"RenderInterval", interval},
        {"RenderMarquee", chosen.marquee}, {"RenderStateRate", rate(1000, interval)},
        {"RenderFSStateRate", rate(660, interval)}, {"RenderPointerRate", rate(100, interval)},
        {"RenderQueueRate", rate(3000, interval)}, {"RenderPlaylistRate", rate(30000, interval)}
    }
    local path = SKIN:GetVariable("@") .. "Performance.Settings.inc"
    for _, value in ipairs(values) do
        SKIN:Bang("!WriteKeyValue", "Variables", value[1], tostring(value[2]), path)
    end
    -- Refresh only loaded player skins; an unloaded compact view stays unloaded.
    SKIN:Bang("!RefreshGroup", "YouTubeMusicPlayer")
end
