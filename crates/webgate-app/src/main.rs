#![forbid(unsafe_code)]

use webgate_browser::BrowserKind;
use webgate_platform::current_platform;
use webgate_transport::TransportState;

fn main() {
    let platform = current_platform();
    let browser = BrowserKind::Servo;
    let transport = TransportState::Stopped;

    println!("WebGate scaffold: platform={platform:?} browser={browser:?} transport={transport:?}");
}
