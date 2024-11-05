# jobby
A composable framework for job applications.

## Motivation

The philosophy behind this project is that you can control and customize your job application using the latest disruptive tools of AI without being a surrogate.

The Problem: for an average job seeker who is trained to use MS Word to house their jpg mugshot and export to pdf is just going to bloat hard drive relatively quickly. Furthermore, discarding everything just to send, send, send doesn't provide you a history of a progressive career arc. While it is trivial to add a sqlite database and call it a day, you need a sensible way to access the data without querying while safely and securely managing private and sensitive information with a secrets management.

The Solution: use a yaml file to define your job application. yaml is used a lot in devops and it's a stringifiable query language while being human readable.

Unlike other tools that are more effective at probably getting you a job (Auto_Jobs_Applier_AIHawk)[https://github.com/feder-cr/Auto_Jobs_Applier_AIHawk]. I am just simply practicing my yaml so I don't have to remember bash commands. While at the same time searching for a job and improving my CV using tools that a dependencies-managed in the framework of CLI compos-ability.

## Usage

### Requirements
- nix
- git

### Installation
- clone the repo, set it to private and the .gitignore to your preferences
- run `nix-shell` to enter a nix shell
- run 'task' for a list of menu options

### Commands
- task generateletter
- task generatecv
- task clean
- exit

## Dependencies
- shrinkpdf.sh is a script to shrink pdf files to a given size. source: https://github.com/andreas-abel/shrinkpdf 
- tgpt
- go-task
- yq-go
- pandoc
- wkhtmltopdf
- texlive.combined.scheme-small
- sops (for secrets management)

## TODO
- [ ] add a way to generate a pdf from a yaml file
- [x] add a way to generate a pdf from a markdown file
- [ ] add a way to parse yaml in sops

## shortcuts to mupdf
1. Navigation:
   - Page Up/Down: Previous/Next page
   - Spacebar: Next page
   - Backspace: Previous page
   - Home: First page
   - End: Last page

2. Zoom:
   - '+' or '=': Increase zoom level
   - '-' or '_' : Decrease zoom level
   - '0': Reset zoom level

3. Text selection:
   - Mouse drag: Select text
   - Shift + Arrow keys: Extend selection

4. Copy/Paste:
   - Ctrl/Cmd + C: Copy selected text
   - Ctrl/Cmd + V: Paste copied text

5. Find:
   - Ctrl/F: Open find panel
   - Ctrl/G: Find next occurrence

6. Bookmarks:
   - Ctrl/D: Add/remove bookmark
   - Ctrl+B: Toggle bookmark visibility

7. Fullscreen:
   - F11: Toggle fullscreen mode

Please note that these shortcuts may vary slightly depending on the operating
 system and MUPDF version. For the most accurate and comprehensive list of
 shortcuts, it's recommended to consult the official MUPDF documentation or help
 menu within the application.Based on the search results, here is a
 comprehensive list of MUPDF shortcuts from the official documentation:

Navigation:
- Page Up/Down: Previous/Next page
- Spacebar: Next page
- Backspace: Previous page
- Home: First page
- End: Last page
- . : Go to next page
- , : Go to previous page
- < : Go back 10 pages
- > : Go forward 10 pages
- [number] g : Go to page number
- G : Go to last page

Zoom/Fit:
- + or = : Zoom in
-  : Zoom out
- W : Fit width
- H : Fit height
- Z : Autofit
- w : Shrinkwrap window to fit page
- [number] z : Set zoom resolution in DPI

Rotation:
- L : Rotate left (counter-clockwise)
- R : Rotate right (clockwise)

Search:
- / : Start searching forward
- ? : Start searching backward
- n/N : Find next/previous search result

Other:
- i : Display document metadata
- o : Toggle outline display
- L : Toggle link display
- r : Reload document
- q : Quit viewer
- f : Toggle fullscreen
- m : Save current page to navigation history
- t/T : Go back/forward in navigation history
- [number] m/t : Set/go to numbered bookmark
- p : Presentation mode
- b : Smart move one screenful backward
- [space] : Smart move one screenful forward
- [comma/page up] : Go one page backward
