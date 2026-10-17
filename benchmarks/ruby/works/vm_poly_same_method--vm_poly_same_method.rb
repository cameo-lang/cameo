module AR; end
class AR::Base
  def create_or_update
    nil
  end
  def save
    create_or_update
  end
end
class Foo < AR::Base; end
class Bar < AR::Base; end
o1 = Foo.new
o2 = Bar.new

o1.save; o2.save;
o1.save; o2.save;
o1.save; o2.save;
o1.save; o2.save;
o1.save; o2.save;
o1.save; o2.save;
o1.save; o2.save;
o1.save; o2.save;

