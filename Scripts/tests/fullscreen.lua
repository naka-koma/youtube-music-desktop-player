-- Lua 5.1 / Rainmeter: dofile('.../Scripts/tests/fullscreen.lua')('.../repo')
return function(root)
    local file=assert(io.open(root .. '/Rainmeter/YouTube Music Desktop Player/@Resources/Fullscreen.lua','rb'))
    local code=file:read('*a');file:close()
    -- The production script is ASCII code stored as Rainmeter UTF-16 LE.
    if code:sub(1,2)==string.char(255,254) then
        code=code:sub(3):gsub('(.)(.)',function(low,high)
            assert(high==string.char(0) and low:byte()<128, 'UTF-16 test fixture must contain ASCII code')
            return low
        end)
    end
    local time,calls=0,{}
    local values={ROOTCONFIG='Player',FullscreenMonitor='1', ['SCREENAREAWIDTH@1']='1920', ['SCREENAREAHEIGHT@1']='1080', ['@']=root .. '/Rainmeter/YouTube Music Desktop Player/@Resources/'}
    local data,pointer='', '0|0|false'
    local function payload(title,position,progress,image,blur,w,h)
        return table.concat({title or '5449544c45','415254495354','',position or '303a3031','true','false',progress or '1','50',image or '',blur or '',w or '0',h or '0',''},'|')
    end
    local env={
        os={clock=function() return time end},
        SKIN={
            GetVariable=function(_,key,default) return values[key] or default end,
            GetMeasure=function(_,name)
                return {GetValue=function() return 1 end,GetStringValue=function()
                    return name=='MeasureFSState' and data or pointer
                end}
            end,
            Bang=function(_, ...)
                local call={...};calls[#calls+1]=call
                if call[1]=='!SetVariable' then values[call[2]]=call[3] end
            end
        }
    }
    setmetatable(env,{__index=_G})
    local chunk=assert(loadstring(code));setfenv(chunk,env);chunk();env.Start()
    local function update()
        calls={};time=time+0.03;env.Update()
    end
    local function contains(bang,target)
        for _,call in ipairs(calls) do
            if call[1]==bang and (not target or call[2]==target) then return true end
        end
        return false
    end
    data=payload();update()
    assert(contains('!UpdateMeasure','MeasureFSTextTitle'), 'initial title must be decoded')
    assert(contains('!UpdateMeterGroup','FSArtwork'), 'initial artwork state must be applied')

    data=payload(nil,'303a3032','2');update()
    assert(contains('!UpdateMeterGroup','FSSeek') and contains('!UpdateMeter','MeterFSTime'), 'progress/time must remain live')
    assert(not contains('!UpdateMeasure','MeasureFSTextTitle'), 'progress must not re-decode unchanged titles')
    assert(not contains('!UpdateMeterGroup','FSArtwork'), 'progress must not rebuild background')
    assert(not contains('!UpdateMeterGroup','FSContent') and not contains('!Redraw'), 'avoid full content updates and duplicate draws')
    update();assert(#calls==0, 'unchanged playback with a still pointer must do no meter work')

    data=payload('4e4557','303a3032','2');update()
    assert(contains('!UpdateMeasure','MeasureFSTextTitle'), 'track changes must refresh the title')
    assert(not contains('!UpdateMeterGroup','FSArtwork'), 'title-only change must not rebuild background')
    data=payload('4e4557','303a3032','2','fullscreen-art-aa.jpg','fullscreen-blur-aa.png','1920','1080');update()
    assert(contains('!UpdateMeterGroup','FSArtwork'), 'artwork must refresh when it arrives asynchronously')
    assert(not contains('!UpdateMeasure','MeasureFSTextTitle'), 'artwork arrival must not reset the title')

    pointer='10|10|false';update()
    assert(contains('!UpdateMeterGroup','FSControls') and not contains('!Redraw'), 'pointer controls use the normal frame draw')
    env.SeekStart(50);env.SeekEnd(50)
    calls={};env.Close();assert(not contains('!DeactivateConfig'), 'seek release must not close fullscreen')
    time=time+0.2;env.Update();calls={};env.Close()
    assert(contains('!DeactivateConfig','Player\\Fullscreen'), 'close guard must expire by elapsed time')
    return 'PASS: selective playback/artwork updates, cached titles, no duplicate redraw, seek close timing'
end
