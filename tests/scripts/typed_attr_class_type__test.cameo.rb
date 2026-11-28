# nested updates: testcase to demonstrate how the user's name attribute
# can be updated via `with { |u| { name: u.name.normalize } }`
#
# (Introduced in PR #465. expected output follows the __END__ separator.)
class UserName
  string :user_name

  def initialize(user_name:)
    self.user_name = user_name
  end

  def normalize = UserName.new(user_name: user_name.strip)
end

class User
  attr :name, type: UserName
end

user = User.new(name: UserName.new(user_name: " hugo "))
normalized_user = user.with { |u| { name: u.name.normalize } }
p normalized_user.name.user_name, user.name.user_name
p normalized_user == User.new(name: UserName.new(user_name: "hugo"))

__END__
"hugo"
" hugo "
true
