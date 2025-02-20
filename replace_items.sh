#!/usr/bin/env bash

# Function to replace items in a document
replace_items_in_document() {
    local filename="$1"
    
    # Check if the file exists
    if [ ! -f "$filename" ]; then
        echo "The file '$filename' was not found."
        return 1
    fi
    
    # Read the first line (header) and store it
    header=$(sed -n '1p' "$filename")
    
    # Process the remaining lines
    modified_content=$(sed -n '2p' "$filename" | awk '{print $0}')
    
    # Build new content line by line, prompting user for input
    new_content="$header\n"
    while IFS= read -r line; do
        read -p "Enter replacement for '$line': " user_input
        if [ -n "$user_input" ]; then
            new_content+="$user_input\n"
        else
            new_content+="$line\n"
        fi
    done <<< "$modified_content"
    
    # Write the new content back to the document
    echo -n "$new_content" > "$filename"
    
    echo "Document updated successfully!"
}
