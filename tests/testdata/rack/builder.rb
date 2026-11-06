require "rack"
res = Rack::Response.new("hi")
res.write(" there")
res.set_header("X-Y", "z")
p res.finish.then { |s, h, b| [s, h, b.to_enum(:each).to_a] }
r = Rack::Response.new
r.write("a")
r.write("b")
p r.finish.then { |s, h, b| [s, h, b.to_enum(:each).to_a] }
q = Rack::Response.new
q.redirect("/there")
p q.finish.then { |s, h, b| [s, h, b.to_enum(:each).to_a] }
inner = ->(env) { [200, {"content-type" => "text/plain"}, ["#{env["SCRIPT_NAME"]}|#{env["PATH_INFO"]}"]] }
app = Rack::Builder.new do
  use Rack::ContentLength
  use Rack::Head
  map "/admin" do
    run inner
  end
  run inner
end
m = Rack::MockRequest.new(app)
["/", "/admin/x", "/y"].each { |path| x = m.get(path); p [x.status, x.headers, x.body] }
p m.head("/").body, m.head("/").headers
begin; Rack::Builder.new { use Rack::Head }.to_app; rescue => e; p [e.class, e.message]; end
