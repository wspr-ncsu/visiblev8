use adblock::engine::Engine;
use adblock::lists::ParseOptions;
use std::io::{self, BufRead, BufReader};
use std::path::Path;
use serde_json::{Value};
use std::fs::File;
use std::env;

fn construct_engine() -> io::Result<Engine> {
    let mut rules = vec![];
    let easy_list_file_name= env::var("EASYLIST_FILE").unwrap_or("easylist.txt".into());
    let easy_privacy_file_name= env::var("EASYPRIVACY_FILE").unwrap_or("easyprivacy.txt".into());
    let rule_path: [&str; 2] = [easy_list_file_name.as_str(), easy_privacy_file_name.as_str()];
    for path in rule_path {
        let file = File::open(path)?;
        let reader = BufReader::new(file);
        let mut lines = reader.lines().collect::<io::Result<Vec<String>>>()?;
        rules.append(&mut lines);
    }

    let engine = Engine::from_rules(&rules, ParseOptions::default());
    Ok(engine)
}

fn read_lines<P>(filename: P) -> io::Result<io::Lines<io::BufReader<File>>>
where P: AsRef<Path>, {
    let file = File::open(filename)?;
    Ok(io::BufReader::new(file).lines())
}

fn main() {
    let second_arg = env::args().nth(1).expect("no second arg");
    let engine = construct_engine().unwrap();
    if let Ok(lines) = read_lines(second_arg) {
        for line in lines {
            let line = line.unwrap();
            let mut v: Value = serde_json::from_str(&line).unwrap();
            if engine.check_network_urls(&v["url"].as_str().unwrap(), &v["origin"].as_str().unwrap(), "script").matched {
                v["blocked"] = true.into()
            } else {
                v["blocked"] = false.into()
            }
            println!("{}",serde_json::to_string(&v).unwrap());
        }
    }
}