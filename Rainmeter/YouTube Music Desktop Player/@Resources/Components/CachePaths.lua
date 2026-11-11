-- Match Go os.TempDir / Windows GetTempPath for the current user.
-- The final Windows-directory fallback is represented by WINDIR.
return function(getenv)
    for _, key in ipairs({"TMP", "TEMP", "USERPROFILE", "WINDIR"}) do
        local directory = getenv(key)
        if directory and directory ~= "" then
            return directory:gsub("[\\/]+$", "") .. "\\YouTubeMusicDesktopPlayer\\artwork\\"
        end
    end
    error("The Windows user temporary directory is unavailable")
end
