class Niller; def nil?; true; end; end
xnil, notnil = nil, Object.new
niller = Niller.new

#--- benchmark: the prelude above runs once, the script below in the loop ---
notnil.nil?
