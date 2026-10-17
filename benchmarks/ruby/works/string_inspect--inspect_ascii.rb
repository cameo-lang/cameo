ascii = "Hello, World! This is a benchmark test string." * 100
utf8 = "こんにちは世界。これはベンチマーク用のテスト文字列です。" * 100
mixed = ("Hello World! " + "テスト" + " is great! ") * 100
binary = ("\xE3\x81\x82" * 100).b
escapy = "\n\t\"\\\#" * 100

#--- benchmark: the prelude above runs once, the script below in the loop ---
ascii.inspect
