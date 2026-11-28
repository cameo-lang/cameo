# json: testcase to demonstrate that JSON dump takes a limit
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
begin
  JSON.dump([[[1]]], 2)
rescue ArgumentError => e
  puts e.message
end

__END__
exceed depth limit
