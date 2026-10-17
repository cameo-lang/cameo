class Niller; def nil?; true; end; end
xnil, notnil = nil, Object.new
niller = Niller.new

niller.nil?
