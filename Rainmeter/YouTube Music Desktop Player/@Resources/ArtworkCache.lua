-- Expose the shared cache path as a string measure for ImageName options.
local directory

function Initialize()
    directory = dofile(SKIN:GetVariable("@") .. "Components/CachePaths.lua")(os.getenv)
end

function Update()
    return directory
end
