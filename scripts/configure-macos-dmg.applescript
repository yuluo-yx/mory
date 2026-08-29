on run arguments
    set diskName to item 1 of arguments

    tell application "Finder"
        tell disk (diskName as text)
            open

            tell container window
                set current view to icon view
                set toolbar visible to false
                set statusbar visible to false
                set pathbar visible to false
                set bounds to {180, 120, 948, 632}
            end tell

            set viewOptions to icon view options of container window
            tell viewOptions
                set arrangement to not arranged
                set icon size to 112
                set text size to 13
                set label position to bottom
            end tell
            set background picture of viewOptions to file ".background:dmg-background.png"

            set position of item "Mory.app" to {190, 260}
            set position of item "Applications" to {578, 260}
            set extension hidden of item "Mory.app" to true

            close
            open
            update without registering applications
            delay 2
        end tell
    end tell
end run
