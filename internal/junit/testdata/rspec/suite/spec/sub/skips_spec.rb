RSpec.describe "skips" do
  it("passes") { expect(true).to be(true) }
  it("is skipped") { skip "example" }
end
