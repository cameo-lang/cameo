f = File
pwd = Dir.pwd
Dir.mkdir('b') unless f.directory?('b')
f.write('b/a', '') unless f.file?('b/a')

relative = 'b/a'
absolute = File.join(pwd, relative)
dir = 'b'
file = 'a'

relative_dir = 'b/c'
absolute_dir = File.join(pwd, relative_dir)
file_dir = 'c'

f.realdirpath(relative_dir, pwd)
