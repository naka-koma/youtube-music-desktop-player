[Variables]
RenderMode=balanced
RenderInterval=66
RenderMarquee=1
RenderStateRate=15
RenderFSStateRate=10
RenderPointerRate=2
RenderQueueRate=45
RenderPlaylistRate=455
@IncludePerformanceUser=#@#Performance.Settings.inc
; Presets override legacy timing values without rewriting personal settings.
@IncludePerformanceProfile=#@#Performance.Profiles\#RenderMode#.inc
