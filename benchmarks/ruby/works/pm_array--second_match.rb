def call(*val)
  case val
    in [String => body]
      [200, {}, [body]]
    in [Integer => status]
      [status, {}, [""]]
    in [Integer, String] => response
      [response[0], {}, [response[1]]]
    in [Integer, Hash, String] => response
      [response[0], response[1], [response[2]]]
  end
end

call(401)
