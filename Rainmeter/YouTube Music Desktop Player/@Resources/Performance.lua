local profiles = {
    {id="power", label="Power Saver"},
    {id="balanced", label="Balanced"},
    {id="smooth", label="Smooth"}
}

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
    local path = SKIN:GetVariable("@") .. "Performance.Settings.inc"
    -- Store the choice only. Versioned presets supply timings on every load,
    -- including when an older settings file still contains derived values.
    SKIN:Bang("!WriteKeyValue", "Variables", "RenderMode", chosen.id, path)
    -- Refresh only loaded player skins; an unloaded compact view stays unloaded.
    SKIN:Bang("!RefreshGroup", "YouTubeMusicPlayer")
end
