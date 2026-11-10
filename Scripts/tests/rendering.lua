-- Lua 5.1 / Rainmeter: dofile('.../Scripts/tests/rendering.lua')('.../repo')
return function(root)
    local resources = root .. '/Rainmeter/YouTube Music Desktop Player/@Resources/'
    local function loadIsolated(name, env)
        setmetatable(env, {__index=_G})
        local chunk = assert(loadfile(resources .. name))
        setfenv(chunk, env)
        chunk()
        return env
    end

    local calls, mode, menuOffset = {}, 'balanced', '0'
    local performance = loadIsolated('Performance.lua', {SKIN={
        GetVariable=function(_, key, default)
            return ({RenderMode=mode, PerformanceMenuOffset=menuOffset, ['@']='resources/'})[key] or default
        end,
        Bang=function(_, ...) calls[#calls+1]={...} end
    }})
    performance.Initialize()
    assert(calls[2][3]=='ContextTitle2' and calls[2][4]=='Balanced (selected)', 'compact selection label')
    calls, menuOffset = {}, '2'
    performance.Initialize()
    assert(calls[1][3]=='ContextTitle3' and calls[2][3]=='ContextTitle4', 'preserve fullscreen menu entries')
    calls = {}
    performance.Select('invalid')
    performance.Select('balanced')
    assert(#calls==0, 'invalid or unchanged profile must not rewrite settings')

    for _, case in ipairs({{'power',100,0,10}, {'smooth',33,1,30}, {'balanced',66,1,15}}) do
        mode, calls = 'other', {}
        performance.Select(case[1])
        local values = {}
        for _, call in ipairs(calls) do
            if call[1]=='!WriteKeyValue' then
                assert(call[2]=='Variables' and call[5]=='resources/Performance.Settings.inc', 'write only personal performance settings')
                values[call[3]]=call[4]
            end
            assert(call[1]~='!ActivateConfig', 'do not activate unloaded skins')
        end
        assert(values.RenderMode==case[1] and tonumber(values.RenderInterval)==case[2], 'profile interval')
        assert(tonumber(values.RenderMarquee)==case[3], 'profile scrolling policy')
        assert(tonumber(values.RenderStateRate)==case[4], 'keep playback polling near one second')
        assert(calls[#calls][1]=='!RefreshGroup' and calls[#calls][2]=='YouTubeMusicPlayer', 'refresh loaded player views after all writes')
    end

    local text, width, viewport, x = 'A long title', 0, 100, 0
    local time, scroll, layoutUpdates, redrawRequests = 0, '1', 0, 0
    local titleMeter = {GetW=function() return width end, SetX=function(_, value) x=value end}
    local marquee = loadIsolated('TitleMarquee.lua', {
        os={clock=function() time=time+0.1; return time end},
        SELF={
            GetOption=function(_, key) return ({TextMeter='title', SourceMeasure='source', ViewportMeter='viewport'})[key] end,
            GetNumberOption=function(_, key, default) return ({PauseSeconds=0.2, ScrollSpeed=100})[key] or default end
        },
        SKIN={
            GetVariable=function(_, key, default) return key=='RenderMarquee' and scroll or default end,
            GetMeter=function(_, key) return key=='title' and titleMeter or {GetW=function() return viewport end} end,
            GetMeasure=function() return {GetStringValue=function() return text end} end,
            Bang=function(_, bang)
                if bang=='!UpdateMeter' then layoutUpdates=layoutUpdates+1; width=#text*20 end
                if bang=='!Redraw' then redrawRequests=redrawRequests+1 end
            end
        }
    })
    marquee.Initialize()
    marquee.Update(); marquee.Update()
    assert(x==0, 'pause at start')
    for i=1,6 do marquee.Update() end
    assert(x<0, 'overflow must scroll')
    local reachedEnd, restarted = false, false
    for i=1,70 do
        marquee.Update()
        assert(x<=0 and x>=-(width-viewport), 'stay within title bounds')
        if x==-(width-viewport) then reachedEnd=true end
        if reachedEnd and x==0 then restarted=true end
    end
    assert(reachedEnd and restarted, 'show the end and return to the start')
    assert(layoutUpdates==1 and redrawRequests==0, 'reuse text layout and normal skin redraw')
    text='Short'; marquee.Update(); marquee.Update()
    for i=1,30 do marquee.Update() end
    assert(x==0, 'fitting titles stay still')
    scroll, text = '0', 'A much longer title'
    for i=1,30 do marquee.Update() end
    assert(x==0, 'Power Saver keeps long titles static')
    scroll='1'
    for i=1,8 do marquee.Update() end
    assert(x<0, 'scrolling resumes when enabled')
    viewport=1000; marquee.Update()
    assert(x==0, 'reset after viewport resize')
    return 'PASS: profiles, menu labels, polling, marquee bounds, caching, static titles, resize'
end
