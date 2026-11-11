-- Lua 5.1 / Rainmeter: dofile('.../Scripts/tests/components.lua')('.../repo')
return function(root)
    local components=root .. '/Rainmeter/YouTube Music Desktop Player/@Resources/Components/'
    local cachePath=dofile(components .. 'CachePaths.lua')
    local env={TMP='C:\\User Temp\\',TEMP='C:\\Other Temp'}
    assert(cachePath(function(key) return env[key] end)=='C:\\User Temp\\YouTubeMusicDesktopPlayer\\artwork\\', 'cache must match Windows TMP precedence')
    env.TMP=''
    assert(cachePath(function(key) return env[key] end)=='C:\\Other Temp\\YouTubeMusicDesktopPlayer\\artwork\\', 'fall back to TEMP when TMP is empty')
    env={USERPROFILE='C:\\User'}
    assert(cachePath(function(key) return env[key] end)=='C:\\User\\YouTubeMusicDesktopPlayer\\artwork\\', 'profile fallback must match Windows')
    assert(not pcall(cachePath,function() return nil end), 'do not create a relative cache path when the environment is missing')
    local newMarquee=dofile(components .. 'Marquee.lua')
    local function travel(step,frames)
        local marquee=newMarquee()
        marquee:step(0,1000,100,0,true)
        for i=1,frames do marquee:step(step,1000,100,0,true) end
        return marquee.offset
    end
    assert(math.abs(travel(0.1,10)-travel(0.01,100))<0.001, 'scroll speed must use elapsed seconds, not frame count')
    local marquee=newMarquee()
    assert(marquee:step(0.1,100,100,0.2,true)==0, 'pause before moving')
    assert(marquee:step(0.1,100,100,0.2,true)==0, 'pause includes the start boundary')
    assert(marquee:step(0.1,100,100,0.2,true)==10, 'advance after pause')
    assert(marquee:step(10,100,100,0.2,true)==35, 'clamp a large wake-up interval')
    assert(marquee:step(0.1,100,100,0.2,false)==0, 'disabling during motion must restore the start')
    assert(marquee:step(0.1,1,100,0.2,true)==0, 'fitting text must stay still')

    local calls={}
    local renderer=dofile(components .. 'Renderer.lua')({Bang=function(_,...) calls[#calls+1]={...} end})
    assert(renderer:move('title',-10.2,0), 'first position must request painting')
    assert(not renderer:move('title',-10.3,0), 'same pixel must not schedule another draw')
    assert(renderer:move('title',-10.8,0), 'next pixel must request painting')
    assert(#calls==2 and calls[1][1]=='!MoveMeter' and calls[2][1]=='!MoveMeter', 'movement must invalidate an all-static skin without text layout or extra Redraw')
    assert(calls[1][2]=='-10' and calls[2][2]=='-11' and calls[2][4]=='title', 'round positions at the renderer boundary')
    renderer:updateMeasure('text')
    renderer:updateMeter('title')
    assert(calls[3][1]=='!UpdateMeasure' and calls[4][1]=='!UpdateMeter', 'decode before rebuilding a changed text layout')
    renderer:updateGroup('FSArtwork')
    assert(calls[5][1]=='!UpdateMeterGroup' and calls[5][2]=='FSArtwork', 'refresh only the requested group')
    return 'PASS: elapsed-time scrolling, static mode reset, draw invalidation, pixel deduplication, scoped layout updates'
end
