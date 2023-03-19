### rmbin
rmbin is a command-line tool that provides a local recycle bin for your files. When you delete a file with rmbin, it is moved to a trash folder instead of being deleted permanently. You can later restore or permanently delete the file from the trash folder as needed.

## Usage
To use rmbin, you can run the following commands:

- `rmbin delete [files...]`: Moves one or more files to the recycle bin. You can specify one or more file paths as arguments.
- `rmbin restore [files...]`: Restores one or more files from the recycle bin. You can specify one or more file paths as arguments.
- `rmbin gc [days]`: Permanently deletes files that have been in the recycle bin for the specified number of days. If no number of days is specified, files will be deleted that are over 30 days old.
- `rmbin help`: Shows help information for the program.

## Configuration
rmbin stores its trash folder and trash map in the user's home directory by default. You can not configure these values now.

## License
This program is licensed under the MIT License. See the LICENSE file for more information.