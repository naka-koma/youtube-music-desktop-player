-- Pure scrolling state; no Rainmeter objects, clocks, fonts, or drawing calls.
return function()
    local marquee = {}

    function marquee:reset()
        self.offset, self.elapsed, self.phase = 0, 0, "start"
        return self.offset
    end

    function marquee:step(seconds, overflow, speed, pause, enabled)
        if not enabled or overflow <= 1 then return self:reset() end
        seconds = math.max(0, math.min(0.25, seconds))
        speed, pause = math.max(1, speed), math.max(0, pause)
        self.elapsed = self.elapsed + seconds
        if self.phase == "start" then
            if self.elapsed >= pause then self.phase, self.elapsed = "scroll", 0 end
        elseif self.phase == "scroll" then
            self.offset = math.min(overflow, self.offset + speed * seconds)
            if self.offset >= overflow then self.phase, self.elapsed = "end", 0 end
        elseif self.elapsed >= pause then
            self:reset()
        end
        return self.offset
    end

    marquee:reset()
    return marquee
end
