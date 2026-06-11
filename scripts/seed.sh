#!/usr/bin/env bash
#
# seed_v2.sh - Seed JobPortal with 100 DC/Marvel/Arrowverse superhero characters.
# Idempotent: safe to run multiple times (ON CONFLICT DO NOTHING).
# Requires: psql (or Docker with postgres container running)
#
set -euo pipefail

COMPOSE_CMD="${COMPOSE_CMD:-}"
SEED_MODE=""

if [ -n "${COMPOSE_CMD}" ] && command -v docker > /dev/null 2>&1 && eval "COMPOSE_CMD exec -T db psql -U jobportal jobportal -c 'SELECT 1'" > /dev/null 2>&1; then
  SEED_MODE="compose"  
  echo "=== Talent Marketplace Seed v2 (via Docker Compose) ==="
elif [ -n "${DATABASE_URL:-}" ] && command -v psql > /dev/null 2>&1; then
  SEED_MODE="direct"
  echo "=== Talent Marketplace Seed v2 (via DATABASE_URL) ==="
elif command -v docker > /dev/null 2>&1 && docker compose exec -T db psql -U jobportal jobportal -c "SELECT 1" > /dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
  SEED_MODE="compose"
  echo "=== Talent Marketplace Seed v2 (via local Docker Compose) ==="
elif command -v psql > /dev/null 2>&1; then
  DATABASE_URL="postgres://jobportal:jobportal@localhost:5432/jobportal?sslmode=disable"
  SEED_MODE="direct"
  echo "=== Talent Marketplace Seed v2 (via localhost fallback) ==="
else
  echo -e "\e[031mERROR\e[0m: No database connection available."
  exit 1
fi

run_psql() {
  if [ "$SEED_MODE" = "compose" ]; then
    eval "$COMPOSE_CMD exec -T db psql -U jobportal jobportal" "$@"
  else
    psql "DATABASE_URL" "$@"
  fi
}

run_sql() {
  run_psql -v ON_ERROR_STOP=1 --no-psqlrc -q "$@"
}
echo "Connecting..."
run_psql -c "SELECT 1" > /dev/null 2>&1 || { echo -e "\e[031mERROR\e[0m: cannot connect"; exit 1;}
echo "Connected."

echo -e "\e[036m[INFO]\e[0m SEED_MODE:$SEED_MODE"
echo -e "\e[036m[INFO]\e[0m COMPOSE_CMD=${COMPOSE_CMD:-<unset>}"
echo -e "\e[036m[INFO]\e[0m DATABASE_URL=${DATABASE_URL:-<unset>}"

PW='$2a$10$Un0k3Q.ot8InE0gJ5j2QLOQz0yCGPAZiwbuSM.I5q3ZDf4nBj7v9O'

###############################################################################
echo "[1/23] Seeding 100 users..."
###############################################################################
run_sql <<EOSQL
DO \$\$
DECLARE
  pw TEXT := '${PW}';
  d_nrcs UUID; d_fs UUID; d_ars UUID; d_aphis UUID; d_fsa UUID; d_rd UUID; d_fns UUID; d_fsis UUID;
BEGIN
  SELECT id INTO d_nrcs FROM departments WHERE name='NRCS' LIMIT 1;
  SELECT id INTO d_fs FROM departments WHERE name='Forest Service' LIMIT 1;
  SELECT id INTO d_ars FROM departments WHERE name='ARS' LIMIT 1;
  SELECT id INTO d_aphis FROM departments WHERE name='APHIS' LIMIT 1;
  SELECT id INTO d_fsa FROM departments WHERE name='FSA' LIMIT 1;
  SELECT id INTO d_rd FROM departments WHERE name='Rural Development' LIMIT 1;
  SELECT id INTO d_fns FROM departments WHERE name='FNS' LIMIT 1;
  SELECT id INTO d_fsis FROM departments WHERE name='FSIS' LIMIT 1;

  -- Justice League
  INSERT INTO users (id,email,password_hash,first_name,last_name,headline,location,role,department_id) VALUES
  (gen_random_uuid(),'bryan.hyland@usda.gov',pw,'Bryan','Hyland','Software Engineer','Silverdale, WA','admin',d_rd),
  (gen_random_uuid(),'clark.kent@usda.gov',pw,'Clark','Kent','IT Specialist & Team Lead | Truth, Justice, and Good Soil Policy','Washington, DC','manager',d_nrcs),
  (gen_random_uuid(),'kara.danvers@usda.gov',pw,'Kara','Danvers','Data Scientist | Stronger than your toughest dataset','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'bruce.wayne@usda.gov',pw,'Bruce','Wayne','Cybersecurity Program Manager | The night shift is my specialty','Washington, DC','manager',d_rd),
  (gen_random_uuid(),'diana.prince@usda.gov',pw,'Diana','Prince','Project Director & Platform Administrator | Leading with wisdom','Washington, DC','admin',d_rd),
  (gen_random_uuid(),'barry.allen@usda.gov',pw,'Barry','Allen','Software Engineer | Fastest deployments in the federal government','Fort Collins, CO','employee',d_nrcs),
  (gen_random_uuid(),'hal.jordan@usda.gov',pw,'Hal','Jordan','Remote Sensing Specialist | The view from orbit is underrated','Salt Lake City, UT','employee',d_nrcs),
  (gen_random_uuid(),'arthur.curry@usda.gov',pw,'Arthur','Curry','Marine Biologist | The tides are changing for conservation','Portland, OR','employee',d_nrcs),
  (gen_random_uuid(),'victor.stone@usda.gov',pw,'Victor','Stone','Systems Engineer | Half man, half machine, all uptime','Kansas City, MO','employee',NULL),
  (gen_random_uuid(),'oliver.queen@usda.gov',pw,'Oliver','Queen','Policy Analyst | You have failed this regulation','Washington, DC','employee',d_fsa),
  (gen_random_uuid(),'dinah.lance@usda.gov',pw,'Dinah','Lance','Communications Specialist | My voice carries','Washington, DC','employee',NULL),
  (gen_random_uuid(),'john.jones@usda.gov',pw,'John','Jones','Intelligence Analyst | I see what others cannot','Ames, IA','employee',d_aphis),
  (gen_random_uuid(),'shayera.hall@usda.gov',pw,'Shayera','Hall','Wildlife Biologist | Protecting species from above','Lakewood, CO','employee',d_fs),
  (gen_random_uuid(),'zatanna.zatara@usda.gov',pw,'Zatanna','Zatara','Training Coordinator | Making complex topics disappear','Washington, DC','employee',d_fs),
  (gen_random_uuid(),'dick.grayson@usda.gov',pw,'Dick','Grayson','IT Project Manager | Agile by nature','Raleigh, NC','employee',d_nrcs),
  (gen_random_uuid(),'barbara.gordon@usda.gov',pw,'Barbara','Gordon','Cybersecurity Analyst | Information is power','Kansas City, MO','employee',d_fs),
  (gen_random_uuid(),'wally.west@usda.gov',pw,'Wally','West','Junior Software Developer | Even faster than Barry','Fort Collins, CO','employee',d_nrcs),
  (gen_random_uuid(),'carter.hall@usda.gov',pw,'Carter','Hall','Archaeologist | Some things are worth preserving forever','Missoula, MT','employee',d_fs),
  (gen_random_uuid(),'ray.palmer@usda.gov',pw,'Ray','Palmer','Research Physicist | The smallest details matter most','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'jefferson.pierce@usda.gov',pw,'Jefferson','Pierce','Electrical Engineer | Powering rural communities','Little Rock, AR','employee',d_rd),
  (gen_random_uuid(),'selina.kyle@usda.gov',pw,'Selina','Kyle','Budget Analyst | I always land on my feet','Washington, DC','employee',d_fsa)
  ON CONFLICT (email) DO NOTHING;

  -- Justice League Dark
  INSERT INTO users (id,email,password_hash,first_name,last_name,headline,location,role,department_id) VALUES
  (gen_random_uuid(),'john.constantine@usda.gov',pw,'John','Constantine','Environmental Compliance | If there is a loophole, I will find it','New Orleans, LA','employee',d_aphis),
  (gen_random_uuid(),'nimue.inwudu@usda.gov',pw,'Nimue','Inwudu','Forecasting Analyst | Predicting outcomes before the data arrives','Washington, DC','employee',d_ars),
  (gen_random_uuid(),'alec.holland@usda.gov',pw,'Alec','Holland','Wetland Ecologist | The Green connects all living systems','Houma, LA','employee',d_nrcs),
  (gen_random_uuid(),'boston.brand@usda.gov',pw,'Boston','Brand','Program Auditor | I see what others overlook','Washington, DC','employee',NULL),
  (gen_random_uuid(),'jason.blood@usda.gov',pw,'Jason','Blood','Historical Records Specialist | Preserving ag heritage since... a long time','Beltsville, MD','employee',NULL)
  ON CONFLICT (email) DO NOTHING;

  -- DC Extended
  INSERT INTO users (id,email,password_hash,first_name,last_name,headline,location,role,department_id) VALUES
  (gen_random_uuid(),'rachel.roth@usda.gov',pw,'Rachel','Roth','Mental Health Counselor | Feeling your way through federal service','Washington, DC','employee',NULL),
  (gen_random_uuid(),'kori.anders@usda.gov',pw,'Kori','Anders','International Ag Liaison | Everything on this planet fascinates me','Washington, DC','employee',NULL),
  (gen_random_uuid(),'garfield.logan@usda.gov',pw,'Garfield','Logan','Veterinary Technician | I understand animals on a personal level','Fort Collins, CO','employee',d_aphis),
  (gen_random_uuid(),'tara.markov@usda.gov',pw,'Tara','Markov','Geologist | I move mountains (of data)','Denver, CO','employee',d_nrcs),
  (gen_random_uuid(),'courtney.whitmore@usda.gov',pw,'Courtney','Whitmore','Public Affairs Intern | Shining a light on USDA stories','Washington, DC','employee',NULL),
  (gen_random_uuid(),'michael.carter@usda.gov',pw,'Michael','Carter','Financial Analyst | I came from the future of budgeting','Washington, DC','employee',d_fsa),
  (gen_random_uuid(),'ted.kord@usda.gov',pw,'Ted','Kord','Entomologist | Bugs are my business','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'kent.nelson@usda.gov',pw,'Kent','Nelson','Senior Policy Advisor | Order must be maintained','Washington, DC','employee',NULL),
  (gen_random_uuid(),'ted.grant@usda.gov',pw,'Ted','Grant','Physical Security Specialist | Old school gets it done','Washington, DC','employee',NULL),
  (gen_random_uuid(),'michael.holt@usda.gov',pw,'Michael','Holt','Data Engineer | Fair play in all things, especially data','Beltsville, MD','manager',d_ars),
  (gen_random_uuid(),'karen.starr@usda.gov',pw,'Karen','Starr','Renewable Energy Specialist | Power to the people','Washington, DC','employee',d_rd),
  (gen_random_uuid(),'helena.bertinelli@usda.gov',pw,'Helena','Bertinelli','OIG Investigator | Justice has no shortcuts','Washington, DC','employee',NULL),
  (gen_random_uuid(),'renee.montoya@usda.gov',pw,'Renee','Montoya','EEO Specialist | The right questions reveal the truth','Washington, DC','employee',NULL),
  (gen_random_uuid(),'mari.mccabe@usda.gov',pw,'Mari','McCabe','Conservation Biologist | The spirit of every species matters','Atlanta, GA','employee',d_fs),
  (gen_random_uuid(),'ralph.dibny@usda.gov',pw,'Ralph','Dibny','Forensic Accountant | I stretch every dollar','Washington, DC','employee',NULL),
  (gen_random_uuid(),'patrick.obrian@usda.gov',pw,'Patrick','OBrian','Outreach Coordinator | Flexible in every situation','Des Moines, IA','employee',d_nrcs),
  (gen_random_uuid(),'nathaniel.adam@usda.gov',pw,'Nathaniel','Adam','Nuclear Safety Officer | Containing the power within','Oak Ridge, TN','employee',d_ars),
  (gen_random_uuid(),'john.irons@usda.gov',pw,'John Henry','Irons','Agricultural Engineer | Building a stronger foundation','Stoneville, MS','employee',d_ars),
  (gen_random_uuid(),'anissa.pierce@usda.gov',pw,'Anissa','Pierce','Civil Rights Attorney | Standing firm for what is right','Washington, DC','employee',NULL),
  (gen_random_uuid(),'jennifer.pierce@usda.gov',pw,'Jennifer','Pierce','Electrical Technician | Sparking change in rural communities','Little Rock, AR','employee',d_rd),
  (gen_random_uuid(),'jaime.reyes@usda.gov',pw,'Jaime','Reyes','IT Support Specialist | The suit does most of the work','El Paso, TX','employee',d_nrcs),
  (gen_random_uuid(),'vic.sage@usda.gov',pw,'Vic','Sage','Investigative Analyst | There is always a deeper answer','Washington, DC','employee',NULL),
  (gen_random_uuid(),'ryan.wilder@usda.gov',pw,'Ryan','Wilder','Plant Protection Specialist | New to the role, ready for anything','Washington, DC','employee',d_aphis),
  (gen_random_uuid(),'luke.fox@usda.gov',pw,'Luke','Fox','DevOps Engineer | Engineering runs in the family','Kansas City, MO','employee',NULL),
  (gen_random_uuid(),'kate.kane@usda.gov',pw,'Kate','Kane','Emergency Management Specialist | Always prepared','Washington, DC','employee',d_aphis)
  ON CONFLICT (email) DO NOTHING;

  -- Arrowverse
  INSERT INTO users (id,email,password_hash,first_name,last_name,headline,location,role,department_id) VALUES
  (gen_random_uuid(),'felicity.smoak@usda.gov',pw,'Felicity','Smoak','Chief Technology Officer | Overqualified and I know it','Washington, DC','manager',NULL),
  (gen_random_uuid(),'john.diggle@usda.gov',pw,'John','Diggle','Veterans Liaison | Service never ends','Washington, DC','employee',NULL),
  (gen_random_uuid(),'thea.queen@usda.gov',pw,'Thea','Queen','Community Development Specialist | Speedy results guaranteed','Washington, DC','employee',d_rd),
  (gen_random_uuid(),'laurel.lance@usda.gov',pw,'Laurel','Lance','Attorney | Fighting for justice in every paragraph','Washington, DC','employee',NULL),
  (gen_random_uuid(),'rene.ramirez@usda.gov',pw,'Rene','Ramirez','Field Inspector | Wild in the field, tame in the office','San Antonio, TX','employee',d_fsis),
  (gen_random_uuid(),'curtis.holt@usda.gov',pw,'Curtis','Holt','AI Research Scientist | Fair play meets machine learning','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'sara.lance@usda.gov',pw,'Sara','Lance','Emergency Response Coordinator | Legends handle the tough calls','Washington, DC','employee',d_aphis),
  (gen_random_uuid(),'cisco.ramon@usda.gov',pw,'Cisco','Ramon','Network Engineer | I name things. It is my thing.','Kansas City, MO','employee',NULL),
  (gen_random_uuid(),'caitlin.snow@usda.gov',pw,'Caitlin','Snow','Cryogenics Researcher | Keeping things cool under pressure','Fort Collins, CO','employee',d_ars),
  (gen_random_uuid(),'iris.west@usda.gov',pw,'Iris','West','Public Affairs Specialist | The story must be told','Washington, DC','employee',NULL),
  (gen_random_uuid(),'joe.west@usda.gov',pw,'Joe','West','Senior Investigator | Trust your gut, verify with data','Washington, DC','employee',NULL),
  (gen_random_uuid(),'nate.heywood@usda.gov',pw,'Nate','Heywood','Agricultural Historian | History has lessons for every harvest','Washington, DC','employee',NULL),
  (gen_random_uuid(),'zari.tomaz@usda.gov',pw,'Zari','Tomaz','Data Privacy Officer | Your data, your rights','Washington, DC','employee',NULL),
  (gen_random_uuid(),'ava.sharpe@usda.gov',pw,'Ava','Sharpe','Program Director | Order, efficiency, results','Washington, DC','manager',NULL),
  (gen_random_uuid(),'gary.green@usda.gov',pw,'Gary','Green','Administrative Assistant | Happy to help! Seriously!','Washington, DC','employee',d_nrcs),
  (gen_random_uuid(),'mary.hamilton@usda.gov',pw,'Mary','Hamilton','Veterinarian | Healing comes naturally','Ames, IA','employee',d_aphis),
  (gen_random_uuid(),'sophie.moore@usda.gov',pw,'Sophie','Moore','Physical Security Manager | Protecting what matters','Washington, DC','employee',NULL),
  (gen_random_uuid(),'mia.queen@usda.gov',pw,'Mia','Queen','Sustainability Coordinator | The next generation of green','Washington, DC','employee',d_nrcs),
  (gen_random_uuid(),'william.clayton@usda.gov',pw,'William','Clayton','Forest Ranger | The forest is my office','Portland, OR','employee',d_fs),
  (gen_random_uuid(),'allegra.garcia@usda.gov',pw,'Allegra','Garcia','Graphic Designer | Light makes everything clearer','Washington, DC','employee',NULL),
  (gen_random_uuid(),'chester.runk@usda.gov',pw,'Chester','Runk','Hardware Engineer | Dense problems need creative solutions','Kansas City, MO','employee',NULL),
  (gen_random_uuid(),'lyla.michaels@usda.gov',pw,'Lyla','Michaels','Intelligence Director | I see the bigger picture','Washington, DC','manager',d_aphis),
  (gen_random_uuid(),'mick.rory@usda.gov',pw,'Mick','Rory','Controlled Burn Specialist | Fire solves everything','Boise, ID','employee',d_fs),
  (gen_random_uuid(),'nora.allen@usda.gov',pw,'Nora','Allen','Child Nutrition Specialist | Every child deserves a good meal','Alexandria, VA','employee',d_fns),
  (gen_random_uuid(),'rory.regan@usda.gov',pw,'Rory','Regan','Textile Inspector | Every thread tells a story','Washington, DC','employee',NULL),
  (gen_random_uuid(),'yao.fei@usda.gov',pw,'Yao','Fei','International Programs Specialist | Survival skills translate to any field','Washington, DC','employee',NULL),
  (gen_random_uuid(),'nyssa.raatko@usda.gov',pw,'Nyssa','Raatko','Plant Quarantine Officer | Nothing gets past me','Miami, FL','employee',d_aphis),
  (gen_random_uuid(),'roy.harper@usda.gov',pw,'Roy','Harper','Substance Abuse Counselor | Recovery is the real strength','Washington, DC','employee',NULL)
  ON CONFLICT (email) DO NOTHING;

  -- X-Men
  INSERT INTO users (id,email,password_hash,first_name,last_name,headline,location,role,department_id) VALUES
  (gen_random_uuid(),'scott.summers@usda.gov',pw,'Scott','Summers','Laser Safety Officer | Focused on precision','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'jean.grey@usda.gov',pw,'Jean','Grey','Organizational Psychologist | I sense great potential in this team','Washington, DC','employee',NULL),
  (gen_random_uuid(),'james.logan@usda.gov',pw,'James','Logan','Timber Management Specialist | I am the best at what I do, bub','Missoula, MT','employee',d_fs),
  (gen_random_uuid(),'ororo.munroe@usda.gov',pw,'Ororo','Munroe','Climatologist | The weather bends to good science','College Park, MD','manager',d_ars),
  (gen_random_uuid(),'charles.xavier@usda.gov',pw,'Charles','Xavier','Training Academy Director | The mind is our greatest resource','Frederick, MD','manager',d_aphis),
  (gen_random_uuid(),'hank.mccoy@usda.gov',pw,'Hank','McCoy','Geneticist | Oh my stars and garters, the data is beautiful','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'bobby.drake@usda.gov',pw,'Bobby','Drake','Cold Storage Specialist | Keeping it cool since day one','Omaha, NE','employee',d_fsis),
  (gen_random_uuid(),'kitty.pryde@usda.gov',pw,'Kitty','Pryde','Systems Administrator | I phase through firewalls','Kansas City, MO','employee',NULL),
  (gen_random_uuid(),'anna.marie@usda.gov',pw,'Anna','Marie','Soil Sampling Technician | One touch and I know everything about this soil','Jackson, MS','employee',d_nrcs),
  (gen_random_uuid(),'remy.lebeau@usda.gov',pw,'Remy','LeBeau','Food Inspector | I have a charged personality','New Orleans, LA','employee',d_fsis),
  (gen_random_uuid(),'jubilation.lee@usda.gov',pw,'Jubilation','Lee','Social Media Manager | Bringing the fireworks to USDA comms','Washington, DC','employee',NULL),
  (gen_random_uuid(),'kurt.wagner@usda.gov',pw,'Kurt','Wagner','Wildlife Tracker | I appear where least expected','Asheville, NC','employee',d_fs),
  (gen_random_uuid(),'emma.frost@usda.gov',pw,'Emma','Frost','Labor Relations Specialist | Crystal clear expectations','Washington, DC','manager',NULL),
  (gen_random_uuid(),'forge@usda.gov',pw,'Forge','Maker','Agricultural Technology Inventor | If it does not exist, I build it','Lubbock, TX','employee',d_ars),
  (gen_random_uuid(),'lorna.dane@usda.gov',pw,'Lorna','Dane','Mineral Soil Scientist | Attracted to the iron in every sample','Tucson, AZ','employee',d_nrcs),
  (gen_random_uuid(),'moira.mactaggert@usda.gov',pw,'Moira','MacTaggert','Epidemiologist | Every outbreak has a pattern','Ames, IA','employee',d_aphis),
  (gen_random_uuid(),'laura.kinney@usda.gov',pw,'Laura','Kinney','Pest Control Specialist | Sharp instincts, sharper solutions','Sacramento, CA','employee',d_aphis),
  (gen_random_uuid(),'betsy.braddock@usda.gov',pw,'Betsy','Braddock','International Trade Analyst | Cutting through trade barriers','Washington, DC','employee',NULL),
  (gen_random_uuid(),'lucas.bishop@usda.gov',pw,'Lucas','Bishop','Law Enforcement Officer | Protecting forests from the future','Washington, DC','employee',d_fs),
  (gen_random_uuid(),'neena.thurman@usda.gov',pw,'Neena','Thurman','Risk Assessment Analyst | Lucky guess? No, lucky analysis.','Washington, DC','employee',d_aphis)
  ON CONFLICT (email) DO NOTHING;

  -- Spider-Verse & remaining
  INSERT INTO users (id,email,password_hash,first_name,last_name,headline,location,role,department_id) VALUES
  (gen_random_uuid(),'peter.parker@usda.gov',pw,'Peter','Parker','Web Developer & Photojournalist | With great server power comes great response-ability','Washington, DC','employee',NULL),
  (gen_random_uuid(),'miles.morales@usda.gov',pw,'Miles','Morales','Junior Data Analyst | New to USDA, ready to learn everything','Brooklyn, NY','employee',d_ars),
  (gen_random_uuid(),'gwen.stacy@usda.gov',pw,'Gwen','Stacy','Biochemist | Science does not care about your hypothesis','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'jessica.drew@usda.gov',pw,'Jessica','Drew','Counterintelligence Analyst | Trust is earned, not given','Washington, DC','employee',d_aphis),
  (gen_random_uuid(),'miguel.ohara@usda.gov',pw,'Miguel','OHara','Innovation Strategist | Building tomorrow agriculture today','Washington, DC','employee',NULL),
  (gen_random_uuid(),'anya.corazon@usda.gov',pw,'Anya','Corazon','Youth Agriculture Educator | Inspiring the next generation','Houston, TX','employee',d_nrcs),
  (gen_random_uuid(),'cindy.moon@usda.gov',pw,'Cindy','Moon','Silk Production Researcher | Weaving science and agriculture','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'may.parker@usda.gov',pw,'May','Parker','Administrative Services Director | Keeping everything together since forever','Washington, DC','employee',d_nrcs),
  (gen_random_uuid(),'ben.reilly@usda.gov',pw,'Ben','Reilly','Lab Technician | Same skills, different perspective','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'cassandra.webb@usda.gov',pw,'Cassandra','Webb','Accessibility Specialist | I see what the interface needs before users do','Washington, DC','employee',NULL),
  (gen_random_uuid(),'dani.moonstar@usda.gov',pw,'Dani','Moonstar','Tribal Liaison | Bridging traditional knowledge and modern agriculture','Albuquerque, NM','employee',d_nrcs),
  (gen_random_uuid(),'amanda.waller@usda.gov',pw,'Amanda','Waller','Program Oversight Director | Results. No excuses.','Washington, DC','manager',NULL),
  (gen_random_uuid(),'pamela.isley@usda.gov',pw,'Pamela','Isley','Botanist | Plants are the answer. Always.','Beltsville, MD','employee',d_ars),
  (gen_random_uuid(),'lois.lane@usda.gov',pw,'Lois','Lane','Senior Writer | The truth deserves a good headline','Washington, DC','employee',NULL)
  ON CONFLICT (email) DO NOTHING;

END \$\$;
EOSQL
echo "  Users seeded."

###############################################################################
echo "[2/23] Seeding skills..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO skills (id, user_id, name)
SELECT gen_random_uuid(), u.id, t.skill
FROM (VALUES
  ('clark.kent@usda.gov','Python'),('clark.kent@usda.gov','SQL'),('clark.kent@usda.gov','Project Management'),('clark.kent@usda.gov','Data Analysis'),('clark.kent@usda.gov','Leadership'),
  ('kara.danvers@usda.gov','Machine Learning'),('kara.danvers@usda.gov','Python'),('kara.danvers@usda.gov','R'),('kara.danvers@usda.gov','Statistics'),('kara.danvers@usda.gov','Data Visualization'),
  ('bruce.wayne@usda.gov','Cybersecurity'),('bruce.wayne@usda.gov','Risk Assessment'),('bruce.wayne@usda.gov','Network Security'),('bruce.wayne@usda.gov','Incident Response'),('bruce.wayne@usda.gov','Python'),
  ('diana.prince@usda.gov','Project Management'),('diana.prince@usda.gov','Leadership'),('diana.prince@usda.gov','Strategic Planning'),('diana.prince@usda.gov','Policy Analysis'),('diana.prince@usda.gov','Agile'),
  ('barry.allen@usda.gov','Go'),('barry.allen@usda.gov','Docker'),('barry.allen@usda.gov','PostgreSQL'),('barry.allen@usda.gov','CI/CD'),('barry.allen@usda.gov','Kubernetes'),('barry.allen@usda.gov','REST APIs'),
  ('hal.jordan@usda.gov','GIS'),('hal.jordan@usda.gov','Remote Sensing'),('hal.jordan@usda.gov','Python'),('hal.jordan@usda.gov','QGIS'),
  ('arthur.curry@usda.gov','Marine Biology'),('arthur.curry@usda.gov','Water Resources'),('arthur.curry@usda.gov','Hydrology'),('arthur.curry@usda.gov','Conservation'),
  ('victor.stone@usda.gov','Systems Engineering'),('victor.stone@usda.gov','AWS'),('victor.stone@usda.gov','Linux'),('victor.stone@usda.gov','Networking'),
  ('oliver.queen@usda.gov','Policy Analysis'),('oliver.queen@usda.gov','Legislation'),('oliver.queen@usda.gov','Public Speaking'),
  ('barbara.gordon@usda.gov','Cybersecurity'),('barbara.gordon@usda.gov','Python'),('barbara.gordon@usda.gov','Penetration Testing'),('barbara.gordon@usda.gov','508 Compliance'),
  ('dick.grayson@usda.gov','Agile'),('dick.grayson@usda.gov','Scrum'),('dick.grayson@usda.gov','Project Management'),('dick.grayson@usda.gov','JIRA'),
  ('wally.west@usda.gov','Go'),('wally.west@usda.gov','JavaScript'),('wally.west@usda.gov','React'),('wally.west@usda.gov','Docker'),
  ('john.constantine@usda.gov','Environmental Law'),('john.constantine@usda.gov','NEPA'),('john.constantine@usda.gov','Wetland Mitigation'),('john.constantine@usda.gov','Regulatory Compliance'),
  ('alec.holland@usda.gov','Wetland Science'),('alec.holland@usda.gov','Soil Science'),('alec.holland@usda.gov','Ecology'),('alec.holland@usda.gov','GIS'),
  ('peter.parker@usda.gov','Web Development'),('peter.parker@usda.gov','Photography'),('peter.parker@usda.gov','JavaScript'),('peter.parker@usda.gov','Go'),
  ('felicity.smoak@usda.gov','Cloud Architecture'),('felicity.smoak@usda.gov','Python'),('felicity.smoak@usda.gov','AWS'),('felicity.smoak@usda.gov','Leadership'),('felicity.smoak@usda.gov','Agile'),
  ('cisco.ramon@usda.gov','Networking'),('cisco.ramon@usda.gov','Python'),('cisco.ramon@usda.gov','SDN'),('cisco.ramon@usda.gov','Automation'),
  ('sara.lance@usda.gov','Emergency Management'),('sara.lance@usda.gov','Incident Command'),('sara.lance@usda.gov','Leadership'),
  ('scott.summers@usda.gov','Laser Safety'),('scott.summers@usda.gov','Physics'),('scott.summers@usda.gov','Lab Management'),
  ('jean.grey@usda.gov','Psychology'),('jean.grey@usda.gov','Organizational Development'),('jean.grey@usda.gov','Training Design'),
  ('james.logan@usda.gov','Timber Management'),('james.logan@usda.gov','Silviculture'),('james.logan@usda.gov','Chainsaw Operations'),('james.logan@usda.gov','Wilderness Survival'),
  ('ororo.munroe@usda.gov','Climatology'),('ororo.munroe@usda.gov','Python'),('ororo.munroe@usda.gov','Weather Modeling'),('ororo.munroe@usda.gov','R'),('ororo.munroe@usda.gov','Leadership'),
  ('charles.xavier@usda.gov','Training Development'),('charles.xavier@usda.gov','Leadership'),('charles.xavier@usda.gov','Psychology'),('charles.xavier@usda.gov','Curriculum Design'),
  ('hank.mccoy@usda.gov','Genetics'),('hank.mccoy@usda.gov','Bioinformatics'),('hank.mccoy@usda.gov','R'),('hank.mccoy@usda.gov','Lab Management'),
  ('miles.morales@usda.gov','Python'),('miles.morales@usda.gov','Data Analysis'),('miles.morales@usda.gov','SQL'),
  ('gwen.stacy@usda.gov','Biochemistry'),('gwen.stacy@usda.gov','Lab Research'),('gwen.stacy@usda.gov','Python'),
  ('michael.holt@usda.gov','Data Engineering'),('michael.holt@usda.gov','Python'),('michael.holt@usda.gov','SQL'),('michael.holt@usda.gov','Spark'),('michael.holt@usda.gov','Leadership'),
  ('emma.frost@usda.gov','Labor Relations'),('emma.frost@usda.gov','Negotiation'),('emma.frost@usda.gov','HR Policy'),('emma.frost@usda.gov','Leadership'),
  ('lyla.michaels@usda.gov','Intelligence Analysis'),('lyla.michaels@usda.gov','Leadership'),('lyla.michaels@usda.gov','Strategic Planning')
) AS t(email, skill)
JOIN users u ON u.email = t.email
WHERE NOT EXISTS (SELECT 1 FROM skills s WHERE s.user_id = u.id AND LOWER(s.name) = LOWER(t.skill));

-- Auto-assign 2 generic skills to any user who has none
INSERT INTO skills (id, user_id, name)
SELECT gen_random_uuid(), u.id, s.skill
FROM users u
CROSS JOIN (VALUES ('Communication'),('Teamwork')) AS s(skill)
WHERE u.email LIKE '%@usda.gov'
  AND NOT EXISTS (SELECT 1 FROM skills sk WHERE sk.user_id = u.id)
  AND NOT EXISTS (SELECT 1 FROM skills sk WHERE sk.user_id = u.id AND sk.name = s.skill);
EOSQL
echo "  Skills seeded."

###############################################################################
echo "[3/23] Seeding connections (~400)..."
###############################################################################
run_sql <<'EOSQL'
-- Tier 1: top 30 characters connect to 10 each
INSERT INTO connections (id, requester_id, addressee_id, status)
SELECT gen_random_uuid(), u1.id, u2.id, 'accepted'
FROM users u1
JOIN users u2 ON u2.id != u1.id
WHERE u1.email IN (
  'clark.kent@usda.gov','bruce.wayne@usda.gov','diana.prince@usda.gov','barry.allen@usda.gov',
  'kara.danvers@usda.gov','hal.jordan@usda.gov','arthur.curry@usda.gov','victor.stone@usda.gov',
  'barbara.gordon@usda.gov','dick.grayson@usda.gov','oliver.queen@usda.gov','dinah.lance@usda.gov',
  'wally.west@usda.gov','john.constantine@usda.gov','alec.holland@usda.gov','felicity.smoak@usda.gov',
  'sara.lance@usda.gov','cisco.ramon@usda.gov','scott.summers@usda.gov','jean.grey@usda.gov',
  'james.logan@usda.gov','ororo.munroe@usda.gov','charles.xavier@usda.gov','hank.mccoy@usda.gov',
  'peter.parker@usda.gov','miles.morales@usda.gov','gwen.stacy@usda.gov','michael.holt@usda.gov',
  'selina.kyle@usda.gov','john.jones@usda.gov'
)
AND u2.email LIKE '%@usda.gov'
AND u1.email < u2.email
AND MOD(abs(hashtext(u1.email || u2.email)), 5) < 2
ON CONFLICT DO NOTHING;

-- Everyone else: connect to 3-5 random Tier 1 characters
INSERT INTO connections (id, requester_id, addressee_id, status)
SELECT gen_random_uuid(), u1.id, u2.id, 'accepted'
FROM users u1
JOIN users u2 ON u2.email IN ('clark.kent@usda.gov','diana.prince@usda.gov','barry.allen@usda.gov','felicity.smoak@usda.gov','ororo.munroe@usda.gov')
WHERE u1.email LIKE '%@usda.gov'
  AND u1.id != u2.id
  AND NOT EXISTS (SELECT 1 FROM connections c WHERE (c.requester_id=u1.id AND c.addressee_id=u2.id) OR (c.requester_id=u2.id AND c.addressee_id=u1.id))
ON CONFLICT DO NOTHING;
EOSQL
echo "  Connections seeded."

###############################################################################
echo "[4/23] Seeding experiences..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO experiences (id, user_id, title, company, location, start_date, end_date, description)
SELECT gen_random_uuid(), u.id, t.title, t.company, t.loc, t.sd::date, t.ed::date, t.descr
FROM (VALUES
  ('clark.kent@usda.gov','IT Specialist','USDA NRCS','Washington, DC','2018-01-01',NULL,'Leading IT modernization across NRCS field offices nationwide.'),
  ('clark.kent@usda.gov','Reporter','Daily Planet Media','Metropolis, KS','2012-06-01','2017-12-31','Award-winning investigative reporting on agricultural policy.'),
  ('bruce.wayne@usda.gov','Cybersecurity Program Manager','USDA OCIO','Washington, DC','2016-03-01',NULL,'Protecting USDA systems through the night. Every night.'),
  ('diana.prince@usda.gov','Project Director','USDA Rural Development','Washington, DC','2015-09-01',NULL,'Directing cross-agency digital transformation initiatives.'),
  ('barry.allen@usda.gov','Software Engineer','USDA NRCS','Fort Collins, CO','2020-01-01',NULL,'Building microservices faster than anyone thought possible.'),
  ('barry.allen@usda.gov','Forensic Analyst','Central City Crime Lab','Central City','2017-03-01','2019-12-31','Applied scientific methods at unprecedented speed.'),
  ('hal.jordan@usda.gov','Remote Sensing Specialist','USDA NRCS','Salt Lake City, UT','2019-06-01',NULL,'Processing satellite imagery for land use classification.'),
  ('hal.jordan@usda.gov','Test Pilot','Ferris Aircraft','Coast City, CA','2014-01-01','2019-05-31','Flight testing with zero fear.'),
  ('arthur.curry@usda.gov','Marine Biologist','USDA NRCS','Portland, OR','2018-08-01',NULL,'Watershed and marine ecosystem conservation.'),
  ('peter.parker@usda.gov','Web Developer','USDA Office of Communications','Washington, DC','2021-06-01',NULL,'Building responsive web applications and taking great photos.'),
  ('peter.parker@usda.gov','Freelance Photographer','Daily Bugle','New York, NY','2018-01-01','2021-05-31','If you want the shot, you have to be in the right place.'),
  ('felicity.smoak@usda.gov','Chief Technology Officer','USDA OCIO','Washington, DC','2017-01-01',NULL,'Overseeing all technology strategy. I am very smart.'),
  ('james.logan@usda.gov','Timber Management Specialist','USDA Forest Service','Missoula, MT','2010-01-01',NULL,'Managing timber resources. I am the best at what I do.'),
  ('ororo.munroe@usda.gov','Climatologist','USDA ARS','College Park, MD','2016-04-01',NULL,'Modeling climate impacts on agricultural systems.'),
  ('john.constantine@usda.gov','Environmental Compliance','USDA APHIS','New Orleans, LA','2019-02-01',NULL,'Finding loopholes in environmental regulations since... always.'),
  ('scott.summers@usda.gov','Laser Safety Officer','USDA ARS','Beltsville, MD','2018-07-01',NULL,'Focused precision in laboratory safety protocols.'),
  ('cisco.ramon@usda.gov','Network Engineer','USDA OCIO','Kansas City, MO','2019-03-01',NULL,'I build networks and name things. Both are important.'),
  ('barbara.gordon@usda.gov','Cybersecurity Analyst','USDA OCIO','Kansas City, MO','2020-01-01',NULL,'Information is power. Protecting it is my mission.'),
  ('hank.mccoy@usda.gov','Geneticist','USDA ARS','Beltsville, MD','2015-01-01',NULL,'Oh my stars and garters, the genomic data is extraordinary.'),
  ('alec.holland@usda.gov','Wetland Ecologist','USDA NRCS','Houma, LA','2017-05-01',NULL,'The swamp is not just a place. It is a living system.')
) AS t(email, title, company, loc, sd, ed, descr)
JOIN users u ON u.email = t.email
WHERE NOT EXISTS (SELECT 1 FROM experiences e WHERE e.user_id = u.id AND e.title = t.title AND e.company = t.company);
EOSQL
echo "  Experiences seeded."

###############################################################################
echo "[5/23] Seeding education..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO educations (id, user_id, school, degree, field_of_study, start_year, end_year)
SELECT gen_random_uuid(), u.id, t.school, t.degree, t.field, t.sy, t.ey
FROM (VALUES
  ('clark.kent@usda.gov','University of Kansas','B.S.','Journalism & Computer Science',2008,2012),
  ('bruce.wayne@usda.gov','Yale University','M.S.','Cybersecurity',2010,2012),
  ('diana.prince@usda.gov','Georgetown University','M.P.A.','Public Administration',2011,2013),
  ('barry.allen@usda.gov','Colorado State University','B.S.','Computer Science',2014,2018),
  ('kara.danvers@usda.gov','MIT','Ph.D.','Data Science',2015,2020),
  ('hal.jordan@usda.gov','University of Utah','M.S.','Remote Sensing',2012,2014),
  ('arthur.curry@usda.gov','Oregon State University','M.S.','Marine Biology',2014,2016),
  ('peter.parker@usda.gov','Empire State University','B.S.','Biophysics & Web Design',2016,2020),
  ('felicity.smoak@usda.gov','MIT','M.S.','Computer Science',2008,2010),
  ('james.logan@usda.gov','University of Montana','Certificate','Forestry Management',2005,2007),
  ('ororo.munroe@usda.gov','Columbia University','Ph.D.','Atmospheric Science',2008,2014),
  ('scott.summers@usda.gov','UC Berkeley','B.S.','Physics',2012,2016),
  ('jean.grey@usda.gov','Harvard University','Ph.D.','Psychology',2010,2016),
  ('charles.xavier@usda.gov','Oxford University','Ph.D.','Education & Psychology',2000,2006),
  ('hank.mccoy@usda.gov','Harvard University','Ph.D.','Genetics',2006,2012),
  ('barbara.gordon@usda.gov','Gotham University','M.S.','Information Security',2016,2018),
  ('cisco.ramon@usda.gov','Stanford University','B.S.','Electrical Engineering',2013,2017),
  ('john.constantine@usda.gov','University of New Orleans','B.A.','Environmental Studies',2005,2009),
  ('alec.holland@usda.gov','LSU','Ph.D.','Wetland Ecology',2008,2014),
  ('miles.morales@usda.gov','Brooklyn College','B.S.','Data Science',2020,2024)
) AS t(email, school, degree, field, sy, ey)
JOIN users u ON u.email = t.email
WHERE NOT EXISTS (SELECT 1 FROM educations ed WHERE ed.user_id = u.id AND ed.school = t.school);
EOSQL
echo "  Education seeded."

###############################################################################
echo "[6/23] Seeding posts (50+)..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO posts (id, user_id, content, created_at)
SELECT gen_random_uuid(), u.id, t.content, NOW() - (t.days || ' days')::interval + (t.hrs || ' hours')::interval
FROM (VALUES
  ('clark.kent@usda.gov','Growing up in Kansas taught me that good soil policy starts at the local level. Every farmer I talk to says the same thing: keep it simple, keep it honest. #Conservation #Kansas',14,9),
  ('clark.kent@usda.gov','Transparency in government technology is not optional. When we build internal tools, the code and the process should be as open as the plains. #OpenGov #USDA',8,11),
  ('bruce.wayne@usda.gov','Updated the USDA threat model at 3 AM. Again. Some of us never sleep. The perimeter is only as strong as its weakest credential. #Cybersecurity #NightShift',13,3),
  ('bruce.wayne@usda.gov','Every agency needs a zero-trust architecture. Trust nothing. Verify everything. Especially USB drives left in parking lots. #InfoSec',7,22),
  ('diana.prince@usda.gov','True leadership means lifting every team member to their full potential. Proud of what this department has accomplished together. #Leadership #USDA',12,10),
  ('diana.prince@usda.gov','Cross-agency collaboration is not a buzzword. It is how we solve problems that no single agency can tackle alone. #OneUSDA',5,14),
  ('barry.allen@usda.gov','Deployed the new microservice in 0.3 seconds. Personal best. The CI/CD pipeline is finally faster than me. Almost. #DevOps #Speed #Go',11,8),
  ('barry.allen@usda.gov','Hot take: Go is the best language for government services. Single binary, no dependencies, compiles in a flash. Literally. #GoLang',4,16),
  ('hal.jordan@usda.gov','The view from Landsat 9 never gets old. New NDVI composites show a 12% increase in cover crop adoption across the Midwest. The data does not lie. #RemoteSensing #GIS',10,7),
  ('arthur.curry@usda.gov','The tides are changing for marine conservation. Our watershed models show a direct link between upstream farming practices and coastal ecosystem health. #WaterResources #Conservation',9,11),
  ('victor.stone@usda.gov','Migrated three legacy systems to containers this week. Uptime: 99.99%. The machines and I have an understanding. #DevOps #Infrastructure',8,15),
  ('oliver.queen@usda.gov','The Farm Bill debate continues. My analysis shows that conservation title funding generates $4 in ecosystem services for every $1 invested. You have failed this budget, Congress. #FarmBill #Policy',7,9),
  ('barbara.gordon@usda.gov','Completed the 508 accessibility audit on the new platform. 47 findings, all resolved. Information should be accessible to everyone. #508Compliance #A11y',6,12),
  ('dick.grayson@usda.gov','Sprint retrospective: 34 story points delivered, zero rollbacks. This team is agile by nature. #Scrum #ProjectManagement',10,13),
  ('peter.parker@usda.gov','With great server power comes great response-ability. Also took some amazing photos of the USDA pollinator garden today. #WebDev #Photography',11,10),
  ('peter.parker@usda.gov','The web of connections on this platform is growing. See what I did there? ...I will see myself out. #WebDevelopment #USDA',3,15),
  ('john.constantine@usda.gov','Found another loophole in the wetland mitigation banking regs. You are welcome. Now if you will excuse me, I need a cigarette and a bourbon. #Compliance #NEPA',9,17),
  ('john.constantine@usda.gov','Spent the day reviewing NEPA environmental assessments. Bloody paperwork would make a demon weep. But it is necessary. Probably. #EnvironmentalLaw',4,8),
  ('alec.holland@usda.gov','The wetlands speak to those who listen. Carbon sequestration rates in our restored marshes exceed projections by 30%. The Green provides. #Wetlands #CarbonSequestration',8,6),
  ('james.logan@usda.gov','Timber sale review up in the Bitterroot. Nobody handles old growth assessment like I do, bub. 47 years of rings tell the whole story. #TimberManagement #ForestService',12,7),
  ('james.logan@usda.gov','Don t call me bub in the standup meeting. Actually, don t call me into the standup meeting at all. I will be in the field. #FieldWork',5,6),
  ('ororo.munroe@usda.gov','The forecast models show a 40% shift in USDA plant hardiness zones by 2040. Farmers need climate-resilient varieties now, not in ten years. We must act. #ClimateChange #Agriculture',10,9),
  ('ororo.munroe@usda.gov','Weather patterns are becoming less predictable. But with better models and better data, we can still help farmers plan. Nature does not have to be the enemy. #Climatology',3,11),
  ('felicity.smoak@usda.gov','Migrated the entire authentication system to... wait, I probably should not tweet the details. Anyway, it is faster now. Much faster. You are welcome. #IT #Security',9,14),
  ('cisco.ramon@usda.gov','Just named the new network monitoring tool "Oracle." Wait, that name is taken. How about "Watchtower"? Every good system needs a cool name. #Networking #Naming',8,10),
  ('scott.summers@usda.gov','Lab safety inspection complete. All laser systems within tolerance. Precision is not just a preference, it is a requirement. #LabSafety #Physics',7,8),
  ('jean.grey@usda.gov','The team dynamics assessment revealed something beautiful: when people feel psychologically safe, innovation follows naturally. I sense great potential here. #OrgPsych #TeamBuilding',6,13),
  ('charles.xavier@usda.gov','Launched the new APHIS training academy curriculum today. The mind is our greatest resource, and investing in training is investing in the future. #Training #APHIS',11,10),
  ('hank.mccoy@usda.gov','Oh my stars and garters! The new CRISPR results on drought-resistant wheat exceeded all expectations. The genome, my friends, is a masterpiece. #Genetics #AgResearch',9,11),
  ('sara.lance@usda.gov','Coordinated the multi-agency response drill today. 200 personnel, 8 agencies, 4 time zones. Legends handle the logistics. #EmergencyResponse',6,7),
  ('kara.danvers@usda.gov','Our crop yield prediction model hit 96% accuracy! Data science is super... I mean, really powerful for agriculture. #MachineLearning #DataScience',7,12),
  ('miles.morales@usda.gov','First week as a data analyst at USDA and I already love it. The datasets here are incredible. Anyone have tips for large-scale PostgreSQL queries? #NewEmployee #DataAnalysis',2,9),
  ('gwen.stacy@usda.gov','Breakthrough in the biochem lab today. New biocontrol agent shows 89% efficacy against soybean aphids with zero environmental residue. Science wins. #Biochemistry #BioControl',5,10),
  ('selina.kyle@usda.gov','The FY2027 budget is taking shape. I always land on my feet, even when OMB throws curveballs. #Budget #FSA',4,14),
  ('dinah.lance@usda.gov','The Secretary press conference went perfectly. My voice carries, even in a room full of reporters. Clear communication saves lives... and programs. #Communications #USDA',6,8),
  ('wally.west@usda.gov','Pushed 47 commits today. Barry says I am even faster than him at code reviews. He is wrong but I appreciate the encouragement. #GoLang #DevOps',3,16),
  ('michael.holt@usda.gov','Fair play in data engineering means clean pipelines, documented transformations, and reproducible results. Built a new Spark job that processes 10TB in 8 minutes. #DataEngineering',8,9),
  ('lyla.michaels@usda.gov','Intelligence briefing delivered to the Secretary on emerging plant health threats. What we do in the shadows keeps agriculture safe in the light. #APHIS #PlantHealth',5,7),
  ('emma.frost@usda.gov','Negotiated the new telework policy for USDA employees. Crystal clear expectations lead to crystal clear results. #HR #LaborRelations',4,11),
  ('ray.palmer@usda.gov','The smallest details matter most. Our new nanoscale sensor detects soil contamination at parts per trillion. Size is relative. #Physics #AgResearch',7,13)
) AS t(email, content, days, hrs)
JOIN users u ON u.email = t.email
WHERE NOT EXISTS (SELECT 1 FROM posts p WHERE p.user_id = u.id AND p.content = t.content);
EOSQL
echo "  Posts seeded."

###############################################################################
echo "[7/23] Seeding likes and comments..."
###############################################################################
run_sql <<'EOSQL'
-- Auto-like: each post gets likes from ~40% of seed users
WITH seed_posts AS (
  SELECT p.id AS post_id, p.user_id AS author_id, ROW_NUMBER() OVER (ORDER BY p.created_at) AS pn
  FROM posts p JOIN users u ON u.id = p.user_id WHERE u.email LIKE '%@usda.gov'
),
seed_users AS (
  SELECT id AS uid, ROW_NUMBER() OVER (ORDER BY email) AS un FROM users WHERE email LIKE '%@usda.gov'
)
INSERT INTO post_likes (id, post_id, user_id, reaction_type)
SELECT gen_random_uuid(), sp.post_id, su.uid,
  CASE MOD(su.un + sp.pn, 10)
    WHEN 0 THEN 'celebrate' WHEN 1 THEN 'insightful' ELSE 'like'
  END
FROM seed_posts sp CROSS JOIN seed_users su
WHERE su.uid != sp.author_id AND MOD(abs(hashtext(sp.post_id::text || su.uid::text)), 5) < 2
ON CONFLICT (post_id, user_id) DO NOTHING;

-- Hand-crafted comments
INSERT INTO comments (id, post_id, user_id, content)
SELECT gen_random_uuid(), p.id, u.id, t.comment
FROM (VALUES
  ('clark.kent@usda.gov','Growing up in Kansas','barry.allen@usda.gov','Kansas produces great engineers too. Just saying.'),
  ('clark.kent@usda.gov','Growing up in Kansas','diana.prince@usda.gov','Well said, Clark. Local voices drive the best policy.'),
  ('bruce.wayne@usda.gov','Updated the USDA threat','barbara.gordon@usda.gov','I will run the penetration test results by you tomorrow.'),
  ('bruce.wayne@usda.gov','Updated the USDA threat','victor.stone@usda.gov','The firewall rules are tight. I double-checked at 3:01 AM.'),
  ('barry.allen@usda.gov','Deployed the new micro','wally.west@usda.gov','0.3 seconds? I can beat that. Watch me.'),
  ('barry.allen@usda.gov','Deployed the new micro','cisco.ramon@usda.gov','I am calling this deployment pipeline "The Speed Force." You are welcome.'),
  ('barry.allen@usda.gov','Hot take: Go is the best','peter.parker@usda.gov','As a fellow web developer, I approve this message. Go is amazing.'),
  ('peter.parker@usda.gov','With great server power','barry.allen@usda.gov','Response-ability. I see what you did there. Respect.'),
  ('peter.parker@usda.gov','With great server power','miles.morales@usda.gov','The pollinator garden photos are incredible! Can you teach me?'),
  ('john.constantine@usda.gov','Found another loophole','alec.holland@usda.gov','Constantine, the wetlands thank you. Reluctantly.'),
  ('john.constantine@usda.gov','Found another loophole','diana.prince@usda.gov','John, please submit the findings through proper channels this time.'),
  ('james.logan@usda.gov','Don t call me bub','ororo.munroe@usda.gov','Logan, the standup meeting is mandatory. Even for you.'),
  ('james.logan@usda.gov','Don t call me bub','scott.summers@usda.gov','Protocol exists for a reason, Logan.'),
  ('ororo.munroe@usda.gov','The forecast models','kara.danvers@usda.gov','Our ML models confirm these projections. The data is clear.'),
  ('ororo.munroe@usda.gov','The forecast models','alec.holland@usda.gov','The wetlands feel the shift already. We must protect them.'),
  ('hank.mccoy@usda.gov','Oh my stars and garters','kara.danvers@usda.gov','The genomic data pairs beautifully with our yield prediction model!'),
  ('felicity.smoak@usda.gov','Migrated the entire','cisco.ramon@usda.gov','You definitely should not tweet the details. But also... how fast?'),
  ('diana.prince@usda.gov','True leadership','clark.kent@usda.gov','Could not agree more. This team inspires me every day.'),
  ('sara.lance@usda.gov','Coordinated the multi','oliver.queen@usda.gov','Impressive coordination. The policy team is ready to support.'),
  ('miles.morales@usda.gov','First week as a data','kara.danvers@usda.gov','Welcome Miles! Happy to help with PostgreSQL — it is my favorite database.')
) AS t(post_author, post_start, commenter, comment)
JOIN users au ON au.email = t.post_author
JOIN posts p ON p.user_id = au.id AND p.content LIKE t.post_start || '%'
JOIN users u ON u.email = t.commenter
WHERE NOT EXISTS (SELECT 1 FROM comments c WHERE c.post_id = p.id AND c.user_id = u.id AND c.content = t.comment);
EOSQL
echo "  Likes and comments seeded."

###############################################################################
echo "[8/23] Seeding postings (10)..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO postings (id, author_id, title, description, type, location, department, location_type, status)
SELECT gen_random_uuid(), u.id, t.title, t.descr, t.ptype, t.loc, t.dept, t.ltype, 'active'
FROM (VALUES
  ('clark.kent@usda.gov','Cloud Migration Engineer','Looking for an engineer to migrate NRCS legacy systems to AWS GovCloud. FedRAMP experience required. 12-month detail.','detail','Fort Collins, CO','NRCS','hybrid'),
  ('clark.kent@usda.gov','Soil Health Dashboard Project','Build an interactive dashboard for soil health metrics. Need GIS and data visualization skills. 6-month project.','project','Washington, DC','NRCS','onsite'),
  ('bruce.wayne@usda.gov','Cybersecurity Incident Response Lead','Senior analyst to lead incident response. CISSP required. Must be comfortable working nights.','detail','Kansas City, MO','OCIO','onsite'),
  ('bruce.wayne@usda.gov','Zero Trust Architecture Implementation','Project to implement zero-trust security across USDA networks. 9-month engagement.','project','Washington, DC','OCIO','hybrid'),
  ('diana.prince@usda.gov','Platform Tester - JobPortal QA','Detail to test the new USDA JobPortal before agency-wide launch. Web app testing experience needed.','detail','Washington, DC','Rural Development','onsite'),
  ('ororo.munroe@usda.gov','Climate Resilience Data Scientist','Seeking a data scientist to model climate impacts on crop yields. Python and R required.','detail','College Park, MD','ARS','hybrid'),
  ('felicity.smoak@usda.gov','AI Ethics Policy Analyst','Develop ethical AI guidelines for USDA applications. Policy and technology background needed.','detail','Washington, DC','OCIO','remote'),
  ('charles.xavier@usda.gov','Training Content Developer','Create e-learning modules for the new employee onboarding system. Instructional design experience required.','detail','Frederick, MD','APHIS','remote'),
  ('michael.holt@usda.gov','Data Pipeline Engineer','Build ETL pipelines for agricultural research data. Spark and Python experience required.','detail','Beltsville, MD','ARS','hybrid'),
  ('clark.kent@usda.gov','GIS Web Application Developer','Build an interactive web map for soil survey data. Leaflet.js and PostGIS. Completed.','project','Fort Collins, CO','NRCS','onsite')
) AS t(email, title, descr, ptype, loc, dept, ltype)
JOIN users u ON u.email = t.email
WHERE NOT EXISTS (SELECT 1 FROM postings p WHERE p.title = t.title);

-- Ensure location_type is populated for this seeded title set on re-runs too.
UPDATE postings p
SET location_type = t.ltype
FROM (VALUES
  ('Cloud Migration Engineer','hybrid'),
  ('Soil Health Dashboard Project','onsite'),
  ('Cybersecurity Incident Response Lead','onsite'),
  ('Zero Trust Architecture Implementation','hybrid'),
  ('Platform Tester - JobPortal QA','onsite'),
  ('Climate Resilience Data Scientist','hybrid'),
  ('AI Ethics Policy Analyst','remote'),
  ('Training Content Developer','remote'),
  ('Data Pipeline Engineer','hybrid'),
  ('GIS Web Application Developer','onsite')
) AS t(title, ltype)
WHERE p.title = t.title
  AND (p.location_type IS NULL OR p.location_type = '' OR p.location_type <> t.ltype);

-- Close the completed one
UPDATE postings SET status = 'closed' WHERE title = 'GIS Web Application Developer';

-- Add skills to postings
INSERT INTO posting_skills (posting_id, skill_name)
SELECT p.id, s.skill FROM postings p
CROSS JOIN LATERAL (VALUES
  ('Cloud Migration Engineer','AWS'),('Cloud Migration Engineer','Docker'),('Cloud Migration Engineer','Kubernetes'),('Cloud Migration Engineer','Terraform'),
  ('Soil Health Dashboard Project','GIS'),('Soil Health Dashboard Project','Python'),('Soil Health Dashboard Project','Data Visualization'),
  ('Cybersecurity Incident Response Lead','Cybersecurity'),('Cybersecurity Incident Response Lead','Incident Response'),('Cybersecurity Incident Response Lead','Python'),
  ('Zero Trust Architecture Implementation','Network Security'),('Zero Trust Architecture Implementation','Cybersecurity'),('Zero Trust Architecture Implementation','AWS'),
  ('Platform Tester - JobPortal QA','Testing'),('Platform Tester - JobPortal QA','Web Development'),('Platform Tester - JobPortal QA','QA'),
  ('Climate Resilience Data Scientist','Python'),('Climate Resilience Data Scientist','R'),('Climate Resilience Data Scientist','Machine Learning'),('Climate Resilience Data Scientist','Statistics'),
  ('AI Ethics Policy Analyst','Policy Analysis'),('AI Ethics Policy Analyst','Python'),('AI Ethics Policy Analyst','Machine Learning'),
  ('Training Content Developer','Training Design'),('Training Content Developer','Instructional Design'),('Training Content Developer','Curriculum Design'),
  ('Data Pipeline Engineer','Python'),('Data Pipeline Engineer','Spark'),('Data Pipeline Engineer','SQL'),('Data Pipeline Engineer','Data Engineering')
) AS s(ptitle, skill) WHERE p.title = s.ptitle
ON CONFLICT (posting_id, skill_name) DO NOTHING;
EOSQL
echo "  Postings seeded."

###############################################################################
echo "[9/23] Seeding news articles (5)..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO news_articles (id, author_id, title, content, published)
SELECT gen_random_uuid(), u.id, t.title, t.content, true
FROM (VALUES
  ('diana.prince@usda.gov','USDA Launches Internal Professional Networking Platform','The USDA today announced the launch of JobPortal, an internal professional networking platform designed to connect employees across all agencies. The platform features skill-matched detail opportunities, real-time messaging, and cross-agency collaboration tools. Built entirely in-house with zero licensing costs, JobPortal represents a new model for government IT innovation.'),
  ('diana.prince@usda.gov','Climate-Smart Agriculture Initiative Receives $3B in Funding','Congress has approved $3 billion in new funding for USDA climate-smart agriculture programs. The initiative supports cover crops, reduced tillage, and improved nutrient management practices. NRCS field offices will begin accepting applications next month.'),
  ('diana.prince@usda.gov','Annual Performance Review Cycle Opens March 15','The FY2026 performance review cycle opens March 15. All supervisors should schedule mid-year check-ins. The new system includes streamlined self-assessments and 360-degree feedback. Use JobPortal accomplishment tracking to document contributions year-round.'),
  ('diana.prince@usda.gov','USDA Innovation Summit - Call for Presentations','The annual USDA Innovation Summit will be held in Washington, DC next month. We are seeking presentations on IT modernization, data science applications, and cross-agency collaboration. Submit proposals through JobPortal by March 31.'),
  ('diana.prince@usda.gov','New Cybersecurity Framework Rolling Out Q2 2026','The Office of the CIO announces an updated cybersecurity framework for all USDA systems. Key changes include mandatory zero-trust architecture, enhanced multi-factor authentication, and improved incident response procedures. Contact the OCIO Security team for details.')
) AS t(email, title, content)
JOIN users u ON u.email = t.email
WHERE NOT EXISTS (SELECT 1 FROM news_articles na WHERE na.title = t.title);
EOSQL
echo "  News seeded."

###############################################################################
echo "[10/23] Seeding employee articles (6)..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO articles (id, author_id, title, content, published)
SELECT gen_random_uuid(), u.id, t.title, t.content, true
FROM (VALUES
  ('barry.allen@usda.gov','Why Go is the Fastest Language for Government Services','Go compiles to a single static binary. No runtime dependencies, no version conflicts. For government environments with rigorous security scanning, this is a game-changer. The standard library handles HTTP, JSON, crypto — all built-in. Fewer dependencies means smaller attack surface. Our team reduced deployment time from hours to seconds. That is not a typo.'),
  ('kara.danvers@usda.gov','Machine Learning for Crop Yield Prediction: A Practical Guide','Start with the data you already have. Years of field observations, weather records, and satellite imagery are perfect for training models. Python with scikit-learn is the fastest path to results. Always validate against held-out test data. And plan for deployment from day one — a model in a notebook helps no one.'),
  ('john.constantine@usda.gov','NEPA Compliance: A Survival Guide','Environmental compliance does not have to be painful. The key is understanding what triggers a full Environmental Impact Statement versus a categorical exclusion. Most projects fall into the latter category if you document properly. And for the love of all that is holy, start the paperwork early.'),
  ('alec.holland@usda.gov','The Case for Wetland Restoration','Wetlands are the kidneys of the landscape. They filter water, sequester carbon, and support biodiversity. Our data shows restored wetlands exceed natural carbon sequestration rates by 30% in the first decade. Every acre of wetland lost costs downstream communities in flood damage and water treatment.'),
  ('ororo.munroe@usda.gov','Climate Adaptation Strategies for American Agriculture','Growing zones are shifting northward at 13 miles per decade. Farmers need climate-resilient crop varieties and adaptive management strategies. Our models show that combining cover crops with reduced tillage can buffer yield losses by up to 25% during drought years.'),
  ('peter.parker@usda.gov','Web Development Best Practices for Government Applications','Accessibility is not optional — Section 508 compliance is law. Use semantic HTML, proper ARIA labels, and test with screen readers. Performance matters: government users often have older hardware. And please, please test in Internet Explorer... just kidding. But do test in every browser your users actually use.')
) AS t(email, title, content)
JOIN users u ON u.email = t.email
WHERE NOT EXISTS (SELECT 1 FROM articles a WHERE a.title = t.title);
EOSQL
echo "  Articles seeded."

###############################################################################
echo "[11/23] Seeding kudos (15)..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO kudos (id, sender_id, receiver_id, message)
SELECT gen_random_uuid(), s.id, r.id, t.msg
FROM (VALUES
  ('clark.kent@usda.gov','barry.allen@usda.gov','Barry, your deployment speed is legendary. The platform launch would not have happened without you.'),
  ('diana.prince@usda.gov','clark.kent@usda.gov','Clark, your steady leadership through the IT modernization has been exemplary. Proud to work alongside you.'),
  ('bruce.wayne@usda.gov','barbara.gordon@usda.gov','Barbara, the security audit was thorough and actionable. Gotham... I mean, USDA is safer because of you.'),
  ('barry.allen@usda.gov','wally.west@usda.gov','Wally, you might actually be faster than me at code reviews. Do not let it go to your head.'),
  ('felicity.smoak@usda.gov','cisco.ramon@usda.gov','Cisco, the network monitoring tool you built is incredible. Even if you did name it something ridiculous.'),
  ('ororo.munroe@usda.gov','kara.danvers@usda.gov','Kara, your ML model accuracy is extraordinary. The agricultural community will benefit enormously.'),
  ('peter.parker@usda.gov','miles.morales@usda.gov','Miles, welcome to USDA! Your eagerness to learn reminds me of myself when I started. You will do great things.'),
  ('james.logan@usda.gov','shayera.hall@usda.gov','Shayera, your raptor survey data was exactly what we needed for the timber sale review. Good work.'),
  ('jean.grey@usda.gov','charles.xavier@usda.gov','Professor, the new training curriculum is transformative. Your vision for employee development is unmatched.'),
  ('cisco.ramon@usda.gov','felicity.smoak@usda.gov','Felicity, I am officially naming you the MVP of the cloud migration. The Overwatch of USDA IT.'),
  ('sara.lance@usda.gov','john.diggle@usda.gov','Dig, your coordination during the emergency drill was flawless. The veterans network is lucky to have you.'),
  ('hank.mccoy@usda.gov','gwen.stacy@usda.gov','Ms. Stacy, your biocontrol research is, if I may say, positively scintillating. Oh my stars and garters.'),
  ('dick.grayson@usda.gov','barry.allen@usda.gov','Barry, 34 story points with zero rollbacks. Your velocity is... well, exactly what I would expect from you.'),
  ('oliver.queen@usda.gov','dinah.lance@usda.gov','Dinah, your press conference preparation was flawless. Your voice truly does carry.'),
  ('john.constantine@usda.gov','alec.holland@usda.gov','Holland, I hate to admit it, but your wetland data saved my compliance report. Cheers, mate.')
) AS t(sender, receiver, msg)
JOIN users s ON s.email = t.sender
JOIN users r ON r.email = t.receiver
WHERE NOT EXISTS (SELECT 1 FROM kudos k WHERE k.sender_id = s.id AND k.receiver_id = r.id AND k.message = t.msg);
EOSQL
echo "  Kudos seeded."

###############################################################################
echo "[12/23] Seeding accomplishments..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO accomplishments (id, user_id, title, description, period_type, period_start, period_end)
SELECT gen_random_uuid(), u.id, t.title, t.descr, t.pt, t.ps::date, t.pe::date
FROM (VALUES
  ('clark.kent@usda.gov','IT Modernization Phase 1','Led migration of 5 legacy NRCS systems achieving FedRAMP authorization.','quarterly','2026-01-01','2026-03-31'),
  ('bruce.wayne@usda.gov','Zero Critical Security Incidents Q1','Maintained zero critical cybersecurity incidents through proactive threat hunting.','quarterly','2026-01-01','2026-03-31'),
  ('diana.prince@usda.gov','JobPortal Platform Launch','Sponsored and directed the launch of the USDA internal networking platform serving 100K employees.','quarterly','2026-01-01','2026-03-31'),
  ('barry.allen@usda.gov','CI/CD Pipeline Optimization','Reduced deployment time from 4 hours to 15 minutes across all NRCS applications.','quarterly','2025-10-01','2025-12-31'),
  ('barbara.gordon@usda.gov','Section 508 Compliance Audit','Completed comprehensive accessibility audit of the JobPortal platform with zero outstanding findings.','quarterly','2026-01-01','2026-03-31'),
  ('ororo.munroe@usda.gov','Climate Adaptation Report','Published the annual climate impact assessment for agricultural growing zones.','yearly','2025-01-01','2025-12-31'),
  ('hank.mccoy@usda.gov','CRISPR Wheat Breakthrough','Published peer-reviewed paper on drought-resistant wheat varieties developed via genome editing.','quarterly','2026-01-01','2026-03-31'),
  ('felicity.smoak@usda.gov','Cloud Migration Strategy','Developed the 3-year cloud migration roadmap adopted by USDA leadership.','yearly','2025-01-01','2025-12-31'),
  ('john.constantine@usda.gov','Wetland Compliance Framework','Rewrote the wetland mitigation compliance framework, closing 12 regulatory gaps.','quarterly','2025-10-01','2025-12-31'),
  ('peter.parker@usda.gov','USDA Web Redesign','Led the responsive redesign of 15 public-facing USDA web pages with full 508 compliance.','quarterly','2026-01-01','2026-03-31')
) AS t(email, title, descr, pt, ps, pe)
JOIN users u ON u.email = t.email
WHERE NOT EXISTS (SELECT 1 FROM accomplishments a WHERE a.user_id = u.id AND a.title = t.title);
EOSQL
echo "  Accomplishments seeded."

###############################################################################
echo "[13/23] Seeding polls (3)..."
###############################################################################
run_sql <<'EOSQL'
DO $$
DECLARE pid UUID; aid UUID; oid UUID; vid UUID;
BEGIN
  SELECT id INTO aid FROM users WHERE email = 'clark.kent@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM polls WHERE question LIKE '%cloud platform%') THEN
    INSERT INTO polls (id, author_id, question, closes_at) VALUES (gen_random_uuid(), aid, 'What is your preferred cloud platform for USDA workloads?', NOW() + INTERVAL '14 days') RETURNING id INTO pid;
    INSERT INTO poll_options (id, poll_id, label, sort_order) VALUES (gen_random_uuid(),pid,'AWS GovCloud',1),(gen_random_uuid(),pid,'Azure Government',2),(gen_random_uuid(),pid,'On-premise',3),(gen_random_uuid(),pid,'Hybrid',4);
    SELECT id INTO oid FROM poll_options WHERE poll_id = pid AND label = 'AWS GovCloud';
    FOR vid IN SELECT id FROM users WHERE email IN ('barry.allen@usda.gov','victor.stone@usda.gov','felicity.smoak@usda.gov','barbara.gordon@usda.gov','cisco.ramon@usda.gov') LOOP
      INSERT INTO poll_votes (poll_id, user_id, option_id) VALUES (pid, vid, oid) ON CONFLICT DO NOTHING;
    END LOOP;
    SELECT id INTO oid FROM poll_options WHERE poll_id = pid AND label = 'Azure Government';
    FOR vid IN SELECT id FROM users WHERE email IN ('dick.grayson@usda.gov','selina.kyle@usda.gov') LOOP
      INSERT INTO poll_votes (poll_id, user_id, option_id) VALUES (pid, vid, oid) ON CONFLICT DO NOTHING;
    END LOOP;
  END IF;

  SELECT id INTO aid FROM users WHERE email = 'ororo.munroe@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM polls WHERE question LIKE '%Innovation Summit%') THEN
    INSERT INTO polls (id, author_id, question, closes_at) VALUES (gen_random_uuid(), aid, 'Which topic for the Innovation Summit keynote?', NOW() + INTERVAL '7 days') RETURNING id INTO pid;
    INSERT INTO poll_options (id, poll_id, label, sort_order) VALUES (gen_random_uuid(),pid,'Climate-Smart Agriculture',1),(gen_random_uuid(),pid,'IT Modernization',2),(gen_random_uuid(),pid,'AI in Government',3),(gen_random_uuid(),pid,'Workforce Development',4);
    SELECT id INTO oid FROM poll_options WHERE poll_id = pid AND label = 'AI in Government';
    FOR vid IN SELECT id FROM users WHERE email IN ('kara.danvers@usda.gov','hank.mccoy@usda.gov','felicity.smoak@usda.gov','curtis.holt@usda.gov') LOOP
      INSERT INTO poll_votes (poll_id, user_id, option_id) VALUES (pid, vid, oid) ON CONFLICT DO NOTHING;
    END LOOP;
    SELECT id INTO oid FROM poll_options WHERE poll_id = pid AND label = 'Climate-Smart Agriculture';
    FOR vid IN SELECT id FROM users WHERE email IN ('arthur.curry@usda.gov','alec.holland@usda.gov','james.logan@usda.gov') LOOP
      INSERT INTO poll_votes (poll_id, user_id, option_id) VALUES (pid, vid, oid) ON CONFLICT DO NOTHING;
    END LOOP;
  END IF;

  SELECT id INTO aid FROM users WHERE email = 'diana.prince@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM polls WHERE question LIKE '%best feature%') THEN
    INSERT INTO polls (id, author_id, question, closes_at) VALUES (gen_random_uuid(), aid, 'What is your favorite JobPortal feature so far?', NOW() + INTERVAL '10 days') RETURNING id INTO pid;
    INSERT INTO poll_options (id, poll_id, label, sort_order) VALUES (gen_random_uuid(),pid,'Chat Messaging',1),(gen_random_uuid(),pid,'Skill-Matched Postings',2),(gen_random_uuid(),pid,'Dark Mode',3),(gen_random_uuid(),pid,'Groups',4);
    SELECT id INTO oid FROM poll_options WHERE poll_id = pid AND label = 'Dark Mode';
    FOR vid IN SELECT id FROM users WHERE email IN ('bruce.wayne@usda.gov','barbara.gordon@usda.gov') LOOP
      INSERT INTO poll_votes (poll_id, user_id, option_id) VALUES (pid, vid, oid) ON CONFLICT DO NOTHING;
    END LOOP;
    SELECT id INTO oid FROM poll_options WHERE poll_id = pid AND label = 'Chat Messaging';
    FOR vid IN SELECT id FROM users WHERE email IN ('cisco.ramon@usda.gov','peter.parker@usda.gov','miles.morales@usda.gov') LOOP
      INSERT INTO poll_votes (poll_id, user_id, option_id) VALUES (pid, vid, oid) ON CONFLICT DO NOTHING;
    END LOOP;
  END IF;
END $$;
EOSQL
echo "  Polls seeded."

###############################################################################
echo "[14/23] Seeding groups and workspaces..."
###############################################################################
run_sql <<'EOSQL'
DO $$
DECLARE gid UUID; cid UUID; mid UUID;
BEGIN
  SELECT id INTO cid FROM users WHERE email = 'barry.allen@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM interest_groups WHERE name = 'Justice League IT Modernization') THEN
    INSERT INTO interest_groups (id, name, description, created_by) VALUES (gen_random_uuid(), 'Justice League IT Modernization', 'Driving technology innovation across USDA agencies.', cid) RETURNING id INTO gid;
    INSERT INTO group_members (group_id, user_id, role) VALUES (gid, cid, 'admin');
    FOR mid IN SELECT id FROM users WHERE email IN ('clark.kent@usda.gov','bruce.wayne@usda.gov','victor.stone@usda.gov','barbara.gordon@usda.gov','felicity.smoak@usda.gov','cisco.ramon@usda.gov','dick.grayson@usda.gov','wally.west@usda.gov','peter.parker@usda.gov','luke.fox@usda.gov') LOOP
      INSERT INTO group_members (group_id, user_id, role) VALUES (gid, mid, 'member') ON CONFLICT DO NOTHING;
    END LOOP;
    INSERT INTO group_posts (id, group_id, user_id, content) VALUES
      (gen_random_uuid(), gid, cid, 'Welcome to the IT Modernization group! Our mission: make USDA tech faster than... well, fast.'),
      (gen_random_uuid(), gid, (SELECT id FROM users WHERE email='cisco.ramon@usda.gov'), 'I am officially naming our Slack alternative "The Watchtower." Nobody can stop me.'),
      (gen_random_uuid(), gid, (SELECT id FROM users WHERE email='bruce.wayne@usda.gov'), 'Security review for all new tools is mandatory. No exceptions. Not even for the Watchtower.');
  END IF;

  SELECT id INTO cid FROM users WHERE email = 'ororo.munroe@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM interest_groups WHERE name = 'Conservation Science Network') THEN
    INSERT INTO interest_groups (id, name, description, created_by) VALUES (gen_random_uuid(), 'Conservation Science Network', 'Connecting researchers across soil, water, wildlife, and climate disciplines.', cid) RETURNING id INTO gid;
    INSERT INTO group_members (group_id, user_id, role) VALUES (gid, cid, 'admin');
    FOR mid IN SELECT id FROM users WHERE email IN ('arthur.curry@usda.gov','alec.holland@usda.gov','hal.jordan@usda.gov','shayera.hall@usda.gov','hank.mccoy@usda.gov','kara.danvers@usda.gov','james.logan@usda.gov','mari.mccabe@usda.gov','kurt.wagner@usda.gov') LOOP
      INSERT INTO group_members (group_id, user_id, role) VALUES (gid, mid, 'member') ON CONFLICT DO NOTHING;
    END LOOP;
    INSERT INTO group_posts (id, group_id, user_id, content) VALUES
      (gen_random_uuid(), gid, cid, 'This month topic: How climate change is shifting USDA plant hardiness zones. Bring data!'),
      (gen_random_uuid(), gid, (SELECT id FROM users WHERE email='alec.holland@usda.gov'), 'The wetlands perspective: every degree of warming costs us 10,000 acres of coastal marsh.');
  END IF;

  SELECT id INTO cid FROM users WHERE email = 'peter.parker@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM interest_groups WHERE name = 'Spider-Web Development Community') THEN
    INSERT INTO interest_groups (id, name, description, created_by) VALUES (gen_random_uuid(), 'Spider-Web Development Community', 'Web developers across USDA sharing tips, tools, and terrible puns.', cid) RETURNING id INTO gid;
    INSERT INTO group_members (group_id, user_id, role) VALUES (gid, cid, 'admin');
    FOR mid IN SELECT id FROM users WHERE email IN ('barry.allen@usda.gov','wally.west@usda.gov','miles.morales@usda.gov','gwen.stacy@usda.gov','felicity.smoak@usda.gov','cisco.ramon@usda.gov') LOOP
      INSERT INTO group_members (group_id, user_id, role) VALUES (gid, mid, 'member') ON CONFLICT DO NOTHING;
    END LOOP;
    INSERT INTO group_posts (id, group_id, user_id, content) VALUES
      (gen_random_uuid(), gid, cid, 'Welcome! Rule 1: All web puns are encouraged. Rule 2: See rule 1.'),
      (gen_random_uuid(), gid, (SELECT id FROM users WHERE email='miles.morales@usda.gov'), 'First post! Excited to learn from all of you. Any Go tutorials you recommend?');
  END IF;

  SELECT id INTO cid FROM users WHERE email = 'charles.xavier@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM interest_groups WHERE name = 'X-Gene Research Collective') THEN
    INSERT INTO interest_groups (id, name, description, created_by) VALUES (gen_random_uuid(), 'X-Gene Research Collective', 'Genetics, genomics, and biotechnology research across USDA labs.', cid) RETURNING id INTO gid;
    INSERT INTO group_members (group_id, user_id, role) VALUES (gid, cid, 'admin');
    FOR mid IN SELECT id FROM users WHERE email IN ('hank.mccoy@usda.gov','jean.grey@usda.gov','scott.summers@usda.gov','ororo.munroe@usda.gov','kara.danvers@usda.gov','emma.frost@usda.gov','forge@usda.gov') LOOP
      INSERT INTO group_members (group_id, user_id, role) VALUES (gid, mid, 'member') ON CONFLICT DO NOTHING;
    END LOOP;
    INSERT INTO group_posts (id, group_id, user_id, content) VALUES
      (gen_random_uuid(), gid, cid, 'The future of agriculture lies in understanding the genome. Together, we will unlock extraordinary potential.'),
      (gen_random_uuid(), gid, (SELECT id FROM users WHERE email='hank.mccoy@usda.gov'), 'Oh my stars and garters — the new CRISPR results are ready for review. Prepare to be amazed.');
  END IF;

  SELECT id INTO cid FROM users WHERE email = 'sara.lance@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM interest_groups WHERE name = 'Arrowverse Emergency Response') THEN
    INSERT INTO interest_groups (id, name, description, created_by) VALUES (gen_random_uuid(), 'Arrowverse Emergency Response', 'Emergency management, rapid response, and interagency coordination.', cid) RETURNING id INTO gid;
    INSERT INTO group_members (group_id, user_id, role) VALUES (gid, cid, 'admin');
    FOR mid IN SELECT id FROM users WHERE email IN ('john.diggle@usda.gov','oliver.queen@usda.gov','lyla.michaels@usda.gov','kate.kane@usda.gov','nyssa.raatko@usda.gov','john.jones@usda.gov') LOOP
      INSERT INTO group_members (group_id, user_id, role) VALUES (gid, mid, 'member') ON CONFLICT DO NOTHING;
    END LOOP;
    INSERT INTO group_posts (id, group_id, user_id, content) VALUES
      (gen_random_uuid(), gid, cid, 'Next drill: March 28. Multi-agency, multi-timezone. Legends do not just respond — we prepare.');
  END IF;
END $$;
EOSQL
echo "  Groups seeded."

run_sql <<'EOSQL'
DO $$
DECLARE wid UUID; cid UUID; mid UUID;
BEGIN
  SELECT id INTO cid FROM users WHERE email = 'barry.allen@usda.gov';
  IF cid IS NOT NULL AND NOT EXISTS (SELECT 1 FROM workspaces WHERE name = 'Rapid Delivery Guild') THEN
    INSERT INTO workspaces (id, name, description, created_by)
    VALUES (gen_random_uuid(), 'Rapid Delivery Guild', 'Engineers focused on agile project management, leadership, Go, Docker, Linux, and reliable delivery practices across USDA.', cid)
    RETURNING id INTO wid;

    INSERT INTO workspace_members (workspace_id, user_id) VALUES (wid, cid) ON CONFLICT DO NOTHING;
    FOR mid IN SELECT id FROM users WHERE email IN ('wally.west@usda.gov','dick.grayson@usda.gov','felicity.smoak@usda.gov','cisco.ramon@usda.gov','peter.parker@usda.gov') LOOP
      INSERT INTO workspace_members (workspace_id, user_id) VALUES (wid, mid) ON CONFLICT DO NOTHING;
    END LOOP;

    INSERT INTO workspace_notes (id, workspace_id, author_id, body) VALUES
      (gen_random_uuid(), wid, cid, 'Welcome to the Rapid Delivery Guild. Share release checklists, runbooks, and postmortems here.'),
      (gen_random_uuid(), wid, (SELECT id FROM users WHERE email='felicity.smoak@usda.gov'), 'Pinned topic for this sprint: deployment rollback drills and monitoring baselines.');
  END IF;

  SELECT id INTO cid FROM users WHERE email = 'ororo.munroe@usda.gov';
  IF cid IS NOT NULL AND NOT EXISTS (SELECT 1 FROM workspaces WHERE name = 'Climate Data Lab') THEN
    INSERT INTO workspaces (id, name, description, created_by)
    VALUES (gen_random_uuid(), 'Climate Data Lab', 'Cross-discipline workspace for data analysis, Python, AI, strategic planning, and policy analysis for climate resilience.', cid)
    RETURNING id INTO wid;

    INSERT INTO workspace_members (workspace_id, user_id) VALUES (wid, cid) ON CONFLICT DO NOTHING;
    FOR mid IN SELECT id FROM users WHERE email IN ('kara.danvers@usda.gov','hank.mccoy@usda.gov','alec.holland@usda.gov','hal.jordan@usda.gov','james.logan@usda.gov') LOOP
      INSERT INTO workspace_members (workspace_id, user_id) VALUES (wid, mid) ON CONFLICT DO NOTHING;
    END LOOP;

    INSERT INTO workspace_notes (id, workspace_id, author_id, body) VALUES
      (gen_random_uuid(), wid, cid, 'Starting thread: climate risk indicators we can standardize across agencies this quarter.'),
      (gen_random_uuid(), wid, (SELECT id FROM users WHERE email='kara.danvers@usda.gov'), 'I will post a first draft model card template by Friday.');
  END IF;

  SELECT id INTO cid FROM users WHERE email = 'charles.xavier@usda.gov';
  IF cid IS NOT NULL AND NOT EXISTS (SELECT 1 FROM workspaces WHERE name = 'Research Methods Studio') THEN
    INSERT INTO workspaces (id, name, description, created_by)
    VALUES (gen_random_uuid(), 'Research Methods Studio', 'Shared workspace for policy analysis, strategic planning, leadership development, communication, and reproducible methods with Python and Rust tooling.', cid)
    RETURNING id INTO wid;

    INSERT INTO workspace_members (workspace_id, user_id) VALUES (wid, cid) ON CONFLICT DO NOTHING;
    FOR mid IN SELECT id FROM users WHERE email IN ('jean.grey@usda.gov','scott.summers@usda.gov','emma.frost@usda.gov','forge@usda.gov','gwen.stacy@usda.gov') LOOP
      INSERT INTO workspace_members (workspace_id, user_id) VALUES (wid, mid) ON CONFLICT DO NOTHING;
    END LOOP;

    INSERT INTO workspace_notes (id, workspace_id, author_id, body) VALUES
      (gen_random_uuid(), wid, cid, 'Please upload current study protocols and note where reproducibility checks are still needed.');
  END IF;

  SELECT id INTO cid FROM users WHERE email = 'sara.lance@usda.gov';
  IF cid IS NOT NULL AND NOT EXISTS (SELECT 1 FROM workspaces WHERE name = 'Incident Readiness Cell') THEN
    INSERT INTO workspaces (id, name, description, created_by)
    VALUES (gen_random_uuid(), 'Incident Readiness Cell', 'Planning workspace for leadership, strategic planning, communication, teamwork, and incident response coordination exercises.', cid)
    RETURNING id INTO wid;

    INSERT INTO workspace_members (workspace_id, user_id) VALUES (wid, cid) ON CONFLICT DO NOTHING;
    FOR mid IN SELECT id FROM users WHERE email IN ('john.diggle@usda.gov','lyla.michaels@usda.gov','kate.kane@usda.gov','john.jones@usda.gov','barbara.gordon@usda.gov') LOOP
      INSERT INTO workspace_members (workspace_id, user_id) VALUES (wid, mid) ON CONFLICT DO NOTHING;
    END LOOP;

    INSERT INTO workspace_notes (id, workspace_id, author_id, body) VALUES
      (gen_random_uuid(), wid, cid, 'Next tabletop: comms outage + severe weather overlap. Draft your runbook updates before Tuesday.');
  END IF;

  -- Keep seeded workspace descriptions aligned with skill-matching keywords on reruns.
  UPDATE workspaces
  SET description = 'Engineers focused on agile project management, leadership, Go, Docker, Linux, and reliable delivery practices across USDA.'
  WHERE name = 'Rapid Delivery Guild';

  UPDATE workspaces
  SET description = 'Cross-discipline workspace for data analysis, Python, AI, strategic planning, and policy analysis for climate resilience.'
  WHERE name = 'Climate Data Lab';

  UPDATE workspaces
  SET description = 'Shared workspace for policy analysis, strategic planning, leadership development, communication, and reproducible methods with Python and Rust tooling.'
  WHERE name = 'Research Methods Studio';

  UPDATE workspaces
  SET description = 'Planning workspace for leadership, strategic planning, communication, teamwork, and incident response coordination exercises.'
  WHERE name = 'Incident Readiness Cell';
END $$;
EOSQL
echo "  Workspaces seeded."

###############################################################################
echo "[15/23] Extracting hashtags..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO hashtags (id, name)
SELECT DISTINCT gen_random_uuid(), LOWER(m[1])
FROM posts p, regexp_matches(p.content, '#(\w+)', 'g') AS m
WHERE NOT EXISTS (SELECT 1 FROM hashtags h WHERE h.name = LOWER(m[1]))
ON CONFLICT (name) DO NOTHING;

INSERT INTO post_hashtags (post_id, hashtag_id)
SELECT DISTINCT p.id, h.id
FROM posts p, regexp_matches(p.content, '#(\w+)', 'g') AS m
JOIN hashtags h ON h.name = LOWER(m[1])
ON CONFLICT (post_id, hashtag_id) DO NOTHING;
EOSQL
echo "  Hashtags extracted."

###############################################################################
echo "[16/23] Seeding conversations & messages..."
###############################################################################
run_sql <<'EOSQL'
DO $$
DECLARE cv UUID; u1 UUID; u2 UUID; u3 UUID; u4 UUID;
BEGIN
  -- Clark & Bruce
  SELECT id INTO u1 FROM users WHERE email='clark.kent@usda.gov';
  SELECT id INTO u2 FROM users WHERE email='bruce.wayne@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=u1 AND cp2.user_id=u2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO cv;
    INSERT INTO conversation_participants VALUES (cv,u1),(cv,u2);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),cv,u1,'Bruce, the security review for the new platform — where do we stand?',NOW()-INTERVAL '3d 9h'),
      (gen_random_uuid(),cv,u2,'Ran the full scan last night. Clean. Your team did good work, Kent.',NOW()-INTERVAL '3d 8h 45m'),
      (gen_random_uuid(),cv,u1,'Appreciate the late hours. You do know you can delegate the 3 AM shifts, right?',NOW()-INTERVAL '3d 8h 30m'),
      (gen_random_uuid(),cv,u2,'I prefer to verify personally. Trust, but verify. Especially at 3 AM.',NOW()-INTERVAL '3d 8h'),
      (gen_random_uuid(),cv,u1,'Fair enough. Diana wants to demo it to leadership next week.',NOW()-INTERVAL '2d 14h'),
      (gen_random_uuid(),cv,u2,'I will have the security brief ready. No vulnerabilities on my watch.',NOW()-INTERVAL '2d 13h 45m');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '2d 13h 45m' WHERE id=cv;
  END IF;

  -- Barry & Cisco
  SELECT id INTO u1 FROM users WHERE email='barry.allen@usda.gov';
  SELECT id INTO u2 FROM users WHERE email='cisco.ramon@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=u1 AND cp2.user_id=u2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO cv;
    INSERT INTO conversation_participants VALUES (cv,u1),(cv,u2);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),cv,u1,'Cisco, the network latency on the dev cluster is killing my deploy times.',NOW()-INTERVAL '2d 10h'),
      (gen_random_uuid(),cv,u2,'How bad are we talking? Because I just optimized the SDN rules.',NOW()-INTERVAL '2d 9h 50m'),
      (gen_random_uuid(),cv,u1,'300ms round trip. That is an ETERNITY.',NOW()-INTERVAL '2d 9h 40m'),
      (gen_random_uuid(),cv,u2,'For you maybe. Normal humans call that fast. But I will look into it.',NOW()-INTERVAL '2d 9h 30m'),
      (gen_random_uuid(),cv,u1,'Thanks. Also, please do not name the new load balancer. We talked about this.',NOW()-INTERVAL '2d 9h'),
      (gen_random_uuid(),cv,u2,'Too late. It is called the Speed Force Balancer. It is already in the config.',NOW()-INTERVAL '2d 8h 45m'),
      (gen_random_uuid(),cv,u1,'...I walked right into that one.',NOW()-INTERVAL '1d 14h');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '1d 14h' WHERE id=cv;
  END IF;

  -- Constantine & Alec Holland
  SELECT id INTO u1 FROM users WHERE email='john.constantine@usda.gov';
  SELECT id INTO u2 FROM users WHERE email='alec.holland@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=u1 AND cp2.user_id=u2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO cv;
    INSERT INTO conversation_participants VALUES (cv,u1),(cv,u2);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),cv,u1,'Holland, I need your wetland delineation report for the Section 404 permit. Yesterday.',NOW()-INTERVAL '4d 11h'),
      (gen_random_uuid(),cv,u2,'The boundaries are complex. Nature does not draw straight lines, Constantine.',NOW()-INTERVAL '4d 10h 45m'),
      (gen_random_uuid(),cv,u1,'Neither does the Army Corps of Engineers, but they still want the paperwork.',NOW()-INTERVAL '4d 10h 30m'),
      (gen_random_uuid(),cv,u2,'I will have the delineation complete by Friday. The hydric soils tell the story.',NOW()-INTERVAL '4d 10h'),
      (gen_random_uuid(),cv,u1,'Brilliant. I owe you a pint. Or whatever swamp ecologists drink.',NOW()-INTERVAL '3d 8h'),
      (gen_random_uuid(),cv,u2,'Water. We drink water. Clean water. That is rather the point.',NOW()-INTERVAL '3d 7h 45m');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '3d 7h 45m' WHERE id=cv;
  END IF;

  -- Logan & Ororo
  SELECT id INTO u1 FROM users WHERE email='james.logan@usda.gov';
  SELECT id INTO u2 FROM users WHERE email='ororo.munroe@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=u1 AND cp2.user_id=u2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO cv;
    INSERT INTO conversation_participants VALUES (cv,u1),(cv,u2);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),cv,u1,'Storm, your climate report says the Bitterroot is getting drier. I can see it in the trees.',NOW()-INTERVAL '2d 7h'),
      (gen_random_uuid(),cv,u2,'The data supports what your instincts are telling you. Precipitation is down 18% over a decade.',NOW()-INTERVAL '2d 6h 45m'),
      (gen_random_uuid(),cv,u1,'We need to adjust the timber harvest plan. The old growth cannot handle the stress.',NOW()-INTERVAL '2d 6h 30m'),
      (gen_random_uuid(),cv,u2,'Agreed. I will present the revised projections at the next planning meeting.',NOW()-INTERVAL '2d 6h'),
      (gen_random_uuid(),cv,u1,'Good. And Ro... thanks for translating my gut feeling into data the bureaucrats understand.',NOW()-INTERVAL '1d 15h');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '1d 15h' WHERE id=cv;
  END IF;

  -- Diana & Felicity
  SELECT id INTO u1 FROM users WHERE email='diana.prince@usda.gov';
  SELECT id INTO u2 FROM users WHERE email='felicity.smoak@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=u1 AND cp2.user_id=u2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO cv;
    INSERT INTO conversation_participants VALUES (cv,u1),(cv,u2);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),cv,u1,'Felicity, the platform launch is next week. Are we ready from the infrastructure side?',NOW()-INTERVAL '1d 10h'),
      (gen_random_uuid(),cv,u2,'Ready? We have been ready for a week. I may have over-engineered the redundancy. Just a little.',NOW()-INTERVAL '1d 9h 50m'),
      (gen_random_uuid(),cv,u1,'Over-engineering reliability is never a flaw. The Secretary will be watching.',NOW()-INTERVAL '1d 9h 40m'),
      (gen_random_uuid(),cv,u2,'No pressure. Just the future of USDA employee engagement on the line. I am fine. Totally fine.',NOW()-INTERVAL '1d 9h 30m'),
      (gen_random_uuid(),cv,u1,'You will be extraordinary. You always are.',NOW()-INTERVAL '1d 9h');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '1d 9h' WHERE id=cv;
  END IF;

  -- Peter & Miles
  SELECT id INTO u1 FROM users WHERE email='peter.parker@usda.gov';
  SELECT id INTO u2 FROM users WHERE email='miles.morales@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=u1 AND cp2.user_id=u2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO cv;
    INSERT INTO conversation_participants VALUES (cv,u1),(cv,u2);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),cv,u2,'Hey Peter! I am starting my first week. Any tips for a new data analyst at USDA?',NOW()-INTERVAL '2d 11h'),
      (gen_random_uuid(),cv,u1,'Welcome Miles! Number one tip: the cafeteria taco Tuesday is secretly amazing.',NOW()-INTERVAL '2d 10h 50m'),
      (gen_random_uuid(),cv,u2,'Ha! Noted. But seriously, any technical advice?',NOW()-INTERVAL '2d 10h 40m'),
      (gen_random_uuid(),cv,u1,'Learn PostgreSQL well. It powers everything here. And do not be afraid to ask questions in the web dev group.',NOW()-INTERVAL '2d 10h 30m'),
      (gen_random_uuid(),cv,u2,'Already joined! Your pun game in there is... something.',NOW()-INTERVAL '2d 10h'),
      (gen_random_uuid(),cv,u1,'I will choose to take that as a compliment. Welcome to the team, Miles.',NOW()-INTERVAL '1d 8h');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '1d 8h' WHERE id=cv;
  END IF;

  -- GROUP: Innovation Summit Planning (Diana, Clark, Felicity, Barry)
  SELECT id INTO u1 FROM users WHERE email='diana.prince@usda.gov';
  SELECT id INTO u2 FROM users WHERE email='clark.kent@usda.gov';
  SELECT id INTO u3 FROM users WHERE email='felicity.smoak@usda.gov';
  SELECT id INTO u4 FROM users WHERE email='barry.allen@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=u1 AND cp2.user_id=u2 AND (SELECT COUNT(*) FROM conversation_participants WHERE conversation_id=cp1.conversation_id)>2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO cv;
    INSERT INTO conversation_participants VALUES (cv,u1),(cv,u2),(cv,u3),(cv,u4);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),cv,u1,'Team, the Innovation Summit agenda needs to be finalized. What are we presenting?',NOW()-INTERVAL '1d 8h'),
      (gen_random_uuid(),cv,u2,'I will cover the IT modernization progress and JobPortal launch.',NOW()-INTERVAL '1d 7h 50m'),
      (gen_random_uuid(),cv,u3,'Cloud migration case study. With actual cost numbers. Leaders love cost numbers.',NOW()-INTERVAL '1d 7h 40m'),
      (gen_random_uuid(),cv,u4,'Live deployment demo. I will ship a feature on stage in under 30 seconds.',NOW()-INTERVAL '1d 7h 30m'),
      (gen_random_uuid(),cv,u1,'Bold. I like it. Let us make this summit one they remember.',NOW()-INTERVAL '1d 7h'),
      (gen_random_uuid(),cv,u3,'I am setting up demo stations in the breakout rooms. Hands-on is key.',NOW()-INTERVAL '12h');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '12h' WHERE id=cv;
  END IF;

  -- GROUP: Security Team (Bruce, Barbara, Victor, Daniel Thomas [if exists])
  SELECT id INTO u1 FROM users WHERE email='bruce.wayne@usda.gov';
  SELECT id INTO u2 FROM users WHERE email='barbara.gordon@usda.gov';
  SELECT id INTO u3 FROM users WHERE email='victor.stone@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=u1 AND cp2.user_id=u2 AND (SELECT COUNT(*) FROM conversation_participants WHERE conversation_id=cp1.conversation_id)>2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO cv;
    INSERT INTO conversation_participants VALUES (cv,u1),(cv,u2),(cv,u3);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),cv,u1,'Team. Phishing attempt detected targeting USDA credentials. Briefing at 0600.',NOW()-INTERVAL '1d 3h'),
      (gen_random_uuid(),cv,u2,'I have the IOCs. Blocking the domains now.',NOW()-INTERVAL '1d 2h 50m'),
      (gen_random_uuid(),cv,u3,'Network telemetry shows zero successful compromises. Containment effective.',NOW()-INTERVAL '1d 2h 40m'),
      (gen_random_uuid(),cv,u1,'Good work. Send the after-action report by end of day.',NOW()-INTERVAL '1d 2h 30m'),
      (gen_random_uuid(),cv,u2,'Already drafting it. Every detail documented.',NOW()-INTERVAL '1d 2h');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '1d 2h' WHERE id=cv;
  END IF;
END $$;
EOSQL
echo "  Conversations seeded."

###############################################################################
echo "[17/23] Seeding endorsements..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO skill_endorsements (id, skill_id, endorser_id)
SELECT gen_random_uuid(), s.id, e.id
FROM (VALUES
  ('barry.allen@usda.gov','Go','clark.kent@usda.gov'),('barry.allen@usda.gov','Go','wally.west@usda.gov'),('barry.allen@usda.gov','Go','peter.parker@usda.gov'),
  ('barry.allen@usda.gov','Docker','victor.stone@usda.gov'),('barry.allen@usda.gov','Docker','felicity.smoak@usda.gov'),
  ('bruce.wayne@usda.gov','Cybersecurity','barbara.gordon@usda.gov'),('bruce.wayne@usda.gov','Cybersecurity','clark.kent@usda.gov'),
  ('barbara.gordon@usda.gov','Cybersecurity','bruce.wayne@usda.gov'),('barbara.gordon@usda.gov','508 Compliance','peter.parker@usda.gov'),
  ('kara.danvers@usda.gov','Machine Learning','hank.mccoy@usda.gov'),('kara.danvers@usda.gov','Python','barry.allen@usda.gov'),
  ('ororo.munroe@usda.gov','Climatology','arthur.curry@usda.gov'),('ororo.munroe@usda.gov','Leadership','diana.prince@usda.gov'),
  ('peter.parker@usda.gov','Web Development','barry.allen@usda.gov'),('peter.parker@usda.gov','Web Development','miles.morales@usda.gov'),
  ('hal.jordan@usda.gov','GIS','alec.holland@usda.gov'),('hal.jordan@usda.gov','Remote Sensing','kara.danvers@usda.gov'),
  ('felicity.smoak@usda.gov','AWS','victor.stone@usda.gov'),('felicity.smoak@usda.gov','Leadership','diana.prince@usda.gov'),
  ('james.logan@usda.gov','Timber Management','shayera.hall@usda.gov'),('james.logan@usda.gov','Wilderness Survival','carter.hall@usda.gov')
) AS t(owner_email, skill_name, endorser_email)
JOIN users su ON su.email = t.owner_email
JOIN skills s ON s.user_id = su.id AND LOWER(s.name) = LOWER(t.skill_name)
JOIN users e ON e.email = t.endorser_email
ON CONFLICT (skill_id, endorser_id) DO NOTHING;
EOSQL
echo "  Endorsements seeded."

###############################################################################
echo "[18/23] Seeding mentorships..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO mentorships (id, mentor_id, mentee_id, status)
SELECT gen_random_uuid(), m.id, mt.id, t.status
FROM (VALUES
  ('clark.kent@usda.gov','barry.allen@usda.gov','active'),
  ('bruce.wayne@usda.gov','barbara.gordon@usda.gov','active'),
  ('diana.prince@usda.gov','dick.grayson@usda.gov','active'),
  ('charles.xavier@usda.gov','jean.grey@usda.gov','completed'),
  ('peter.parker@usda.gov','miles.morales@usda.gov','active'),
  ('felicity.smoak@usda.gov','cisco.ramon@usda.gov','active')
) AS t(mentor, mentee, status)
JOIN users m ON m.email=t.mentor JOIN users mt ON mt.email=t.mentee
WHERE NOT EXISTS (SELECT 1 FROM mentorships ms WHERE ms.mentor_id=m.id AND ms.mentee_id=mt.id);
EOSQL
echo "  Mentorships seeded."

###############################################################################
echo "[19/23] Seeding feedback requests..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO feedback_requests (id, requester_id, reviewer_id, subject, feedback, status)
SELECT gen_random_uuid(), req.id, rev.id, t.subj, t.fb, t.st
FROM (VALUES
  ('barry.allen@usda.gov','clark.kent@usda.gov','CI/CD pipeline performance','Barry has transformed our deployment process. His speed and accuracy are unmatched.','completed'),
  ('barbara.gordon@usda.gov','bruce.wayne@usda.gov','Security audit quality','Thorough, meticulous, and actionable. Barbara sets the standard for security analysis.','completed'),
  ('miles.morales@usda.gov','peter.parker@usda.gov','First quarter onboarding',NULL,'pending'),
  ('peter.parker@usda.gov','diana.prince@usda.gov','Web redesign project','Peter delivered an accessible, performant redesign on time. His humor is a bonus.','completed')
) AS t(req_email, rev_email, subj, fb, st)
JOIN users req ON req.email=t.req_email JOIN users rev ON rev.email=t.rev_email
WHERE NOT EXISTS (SELECT 1 FROM feedback_requests fr WHERE fr.requester_id=req.id AND fr.subject=t.subj);
EOSQL
echo "  Feedback seeded."

###############################################################################
echo "[20/23] Seeding follows..."
###############################################################################
run_sql <<'EOSQL'
INSERT INTO follows (id, follower_id, followed_id)
SELECT gen_random_uuid(), f.id, fl.id
FROM (VALUES
  ('miles.morales@usda.gov','clark.kent@usda.gov'),('miles.morales@usda.gov','peter.parker@usda.gov'),
  ('wally.west@usda.gov','barry.allen@usda.gov'),('garfield.logan@usda.gov','kara.danvers@usda.gov'),
  ('courtney.whitmore@usda.gov','diana.prince@usda.gov'),('jaime.reyes@usda.gov','victor.stone@usda.gov'),
  ('gary.green@usda.gov','sara.lance@usda.gov'),('allegra.garcia@usda.gov','iris.west@usda.gov'),
  ('gwen.stacy@usda.gov','hank.mccoy@usda.gov'),('tara.markov@usda.gov','hal.jordan@usda.gov'),
  ('jennifer.pierce@usda.gov','jefferson.pierce@usda.gov'),('mia.queen@usda.gov','oliver.queen@usda.gov'),
  ('selina.kyle@usda.gov','bruce.wayne@usda.gov'),('bobby.drake@usda.gov','ororo.munroe@usda.gov')
) AS t(f_email, fl_email)
JOIN users f ON f.email=t.f_email JOIN users fl ON fl.email=t.fl_email
ON CONFLICT (follower_id, followed_id) DO NOTHING;
EOSQL
echo "  Follows seeded."

###############################################################################
echo "[21/23] Diversifying reactions..."
###############################################################################
run_sql <<'EOSQL'
WITH ranked AS (SELECT id, ROW_NUMBER() OVER (ORDER BY id) AS rn FROM post_likes WHERE reaction_type='like')
UPDATE post_likes SET reaction_type='celebrate' WHERE id IN (SELECT id FROM ranked WHERE MOD(rn,7)=0);
WITH ranked AS (SELECT id, ROW_NUMBER() OVER (ORDER BY id) AS rn FROM post_likes WHERE reaction_type='like')
UPDATE post_likes SET reaction_type='insightful' WHERE id IN (SELECT id FROM ranked WHERE MOD(rn,9)=0);
EOSQL
echo "  Reactions diversified."

###############################################################################
echo "[22/23] Seeding Bryan Hyland interactions..."
###############################################################################
run_sql <<'EOSQL'
DO $$
DECLARE
  v_bryan UUID; v_other UUID; v_conv UUID; v_post UUID; v_skill UUID;
  v_barry UUID; v_diana UUID; v_felicity UUID; v_victor UUID;
BEGIN
  SELECT id INTO v_bryan FROM users WHERE email='bryan.hyland@usda.gov';
  IF v_bryan IS NULL THEN RAISE NOTICE 'bryan.hyland not found'; RETURN; END IF;

  -- Connect Bryan to 12 characters (pre-connected for demo)
  FOR v_other IN SELECT id FROM users WHERE email IN (
    'clark.kent@usda.gov','bruce.wayne@usda.gov','diana.prince@usda.gov','barry.allen@usda.gov',
    'barbara.gordon@usda.gov','dick.grayson@usda.gov','victor.stone@usda.gov','kara.danvers@usda.gov',
    'felicity.smoak@usda.gov','peter.parker@usda.gov','ororo.munroe@usda.gov','john.constantine@usda.gov'
  ) LOOP
    INSERT INTO connections (id,requester_id,addressee_id,status) VALUES (gen_random_uuid(),v_bryan,v_other,'accepted') ON CONFLICT DO NOTHING;
  END LOOP;

  -- Bryan posts
  INSERT INTO posts (id,user_id,content,created_at)
  SELECT gen_random_uuid(), v_bryan, t.c, NOW()-t.a FROM (VALUES
    ('Just deployed USDA JobPortal — an internal networking platform built with Go, PostgreSQL, and Docker. Zero licensing cost, 55+ features, full 508 compliance. #GovTech #Go #USDA'::text, INTERVAL '7d'-INTERVAL '10h'),
    ('Open source is the future of government IT. No vendor lock-in, no licensing fees, just good engineering. #OpenSource #Innovation', INTERVAL '4d'-INTERVAL '14h'),
    ('Shout out to @Barry Allen for the fastest code reviews in federal history and @Barbara Gordon for the most thorough security audit I have ever seen.', INTERVAL '2d'-INTERVAL '9h'),
    ('The intersection of data science and agriculture is where real innovation happens. Grateful to work with talented people like @Kara Danvers and @Ororo Munroe. #MachineLearning #USDA', INTERVAL '1d'-INTERVAL '11h')
  ) AS t(c,a)
  WHERE NOT EXISTS (SELECT 1 FROM posts p WHERE p.user_id=v_bryan AND p.content=t.c);

  -- Likes on Bryan posts
  FOR v_post IN SELECT id FROM posts WHERE user_id=v_bryan LOOP
    INSERT INTO post_likes (id,post_id,user_id,reaction_type)
    SELECT gen_random_uuid(), v_post, u.id,
      CASE (ROW_NUMBER() OVER ())::int%4 WHEN 0 THEN 'celebrate' WHEN 1 THEN 'insightful' ELSE 'like' END
    FROM users u WHERE u.email IN ('clark.kent@usda.gov','bruce.wayne@usda.gov','diana.prince@usda.gov','barry.allen@usda.gov','barbara.gordon@usda.gov','kara.danvers@usda.gov','felicity.smoak@usda.gov','peter.parker@usda.gov','ororo.munroe@usda.gov','victor.stone@usda.gov')
    ON CONFLICT ON CONSTRAINT post_likes_post_id_user_id_key DO NOTHING;
  END LOOP;

  -- Comments on Bryan platform post
  SELECT id INTO v_post FROM posts WHERE user_id=v_bryan AND content LIKE '%Just deployed%' LIMIT 1;
  IF v_post IS NOT NULL THEN
    SELECT id INTO v_other FROM users WHERE email='clark.kent@usda.gov';
    INSERT INTO comments (id,post_id,user_id,content) SELECT gen_random_uuid(),v_post,v_other,'This is exactly what USDA has been waiting for. Outstanding work, Bryan.' WHERE NOT EXISTS (SELECT 1 FROM comments c WHERE c.post_id=v_post AND c.user_id=v_other);
    SELECT id INTO v_other FROM users WHERE email='barry.allen@usda.gov';
    INSERT INTO comments (id,post_id,user_id,content) SELECT gen_random_uuid(),v_post,v_other,'Go was the right call. Fastest platform I have ever seen in government.' WHERE NOT EXISTS (SELECT 1 FROM comments c WHERE c.post_id=v_post AND c.user_id=v_other);
    SELECT id INTO v_other FROM users WHERE email='diana.prince@usda.gov';
    INSERT INTO comments (id,post_id,user_id,content) SELECT gen_random_uuid(),v_post,v_other,'Can we demo this at the Innovation Summit? Leadership needs to see this.' WHERE NOT EXISTS (SELECT 1 FROM comments c WHERE c.post_id=v_post AND c.user_id=v_other);
    SELECT id INTO v_other FROM users WHERE email='john.constantine@usda.gov';
    INSERT INTO comments (id,post_id,user_id,content) SELECT gen_random_uuid(),v_post,v_other,'Bloody impressive, mate. Even I cannot find a compliance issue. Well done.' WHERE NOT EXISTS (SELECT 1 FROM comments c WHERE c.post_id=v_post AND c.user_id=v_other);
  END IF;

  -- Bryan conversations
  -- Bryan & Clark
  SELECT id INTO v_other FROM users WHERE email='clark.kent@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=v_bryan AND cp2.user_id=v_other AND (SELECT COUNT(*) FROM conversation_participants WHERE conversation_id=cp1.conversation_id)=2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO v_conv;
    INSERT INTO conversation_participants VALUES (v_conv,v_bryan),(v_conv,v_other);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),v_conv,v_other,'Bryan, leadership wants to discuss scaling JobPortal department-wide. Can you prepare a cost brief?',NOW()-INTERVAL '2d 15h'),
      (gen_random_uuid(),v_conv,v_bryan,'Already on it. LinkedIn Enterprise costs $60/user/year. Ours is zero. For 100K users that is $6M saved annually.',NOW()-INTERVAL '2d 14h 50m'),
      (gen_random_uuid(),v_conv,v_other,'Include the FOIA tool and 508 compliance in the brief. Those close the deal with legal.',NOW()-INTERVAL '1d 9h'),
      (gen_random_uuid(),v_conv,v_bryan,'Both built and tested. Full FOIA search for admins, complete 508 with focus states, contrast, screen readers.',NOW()-INTERVAL '1d 8h 50m'),
      (gen_random_uuid(),v_conv,v_other,'You have thought of everything. Demo is Tuesday.',NOW()-INTERVAL '1d 8h 30m');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '1d 8h 30m' WHERE id=v_conv;
  END IF;

  -- Bryan & Bruce
  SELECT id INTO v_other FROM users WHERE email='bruce.wayne@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=v_bryan AND cp2.user_id=v_other AND (SELECT COUNT(*) FROM conversation_participants WHERE conversation_id=cp1.conversation_id)=2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO v_conv;
    INSERT INTO conversation_participants VALUES (v_conv,v_bryan),(v_conv,v_other);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),v_conv,v_other,'Hyland. Ran the security scan on JobPortal at 2 AM. Results are in.',NOW()-INTERVAL '3d 22h'),
      (gen_random_uuid(),v_conv,v_bryan,'And? Don t leave me in suspense, Bruce.',NOW()-INTERVAL '3d 14h'),
      (gen_random_uuid(),v_conv,v_other,'Clean. Zero critical findings. Your BYTEA storage for files was a smart call. No path traversal possible.',NOW()-INTERVAL '3d 13h 50m'),
      (gen_random_uuid(),v_conv,v_bryan,'That was intentional. No filesystem, no filesystem attacks. Everything lives in PostgreSQL.',NOW()-INTERVAL '3d 13h 40m'),
      (gen_random_uuid(),v_conv,v_other,'Noted. I will sign off on the ATO. Barbara is reviewing the 508 compliance report separately.',NOW()-INTERVAL '2d 3h');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '2d 3h' WHERE id=v_conv;
  END IF;

  -- Bryan & Barry
  SELECT id INTO v_other FROM users WHERE email='barry.allen@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=v_bryan AND cp2.user_id=v_other AND (SELECT COUNT(*) FROM conversation_participants WHERE conversation_id=cp1.conversation_id)=2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO v_conv;
    INSERT INTO conversation_participants VALUES (v_conv,v_bryan),(v_conv,v_other);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),v_conv,v_other,'Bryan! Love the Go codebase. Clean, fast, well-structured. Want to pair on the SSE optimization?',NOW()-INTERVAL '3d 11h'),
      (gen_random_uuid(),v_conv,v_bryan,'Absolutely. The notification streaming works but typing indicators need separate event types.',NOW()-INTERVAL '3d 10h 50m'),
      (gen_random_uuid(),v_conv,v_other,'Use named SSE events. event: typing vs event: message. Client can distinguish them easily.',NOW()-INTERVAL '3d 10h 30m'),
      (gen_random_uuid(),v_conv,v_bryan,'Perfect. I also need to fix the read receipt epoch bug. The COALESCE with epoch returns 1970, not zero in Go.',NOW()-INTERVAL '3d 10h'),
      (gen_random_uuid(),v_conv,v_other,'Classic. Use a sentinel check for year <= 1970 instead of IsZero. I have hit that exact bug before.',NOW()-INTERVAL '2d 16h'),
      (gen_random_uuid(),v_conv,v_bryan,'You are a lifesaver, Barry. Fastest bug fix in federal history.',NOW()-INTERVAL '2d 15h 45m');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '2d 15h 45m' WHERE id=v_conv;
  END IF;

  -- Bryan GROUP conv: Innovation brainstorm
  SELECT id INTO v_barry FROM users WHERE email='barry.allen@usda.gov';
  SELECT id INTO v_diana FROM users WHERE email='diana.prince@usda.gov';
  SELECT id INTO v_felicity FROM users WHERE email='felicity.smoak@usda.gov';
  SELECT id INTO v_victor FROM users WHERE email='victor.stone@usda.gov';
  IF NOT EXISTS (SELECT 1 FROM conversation_participants cp1 JOIN conversation_participants cp2 ON cp1.conversation_id=cp2.conversation_id WHERE cp1.user_id=v_bryan AND cp2.user_id=v_diana AND (SELECT COUNT(*) FROM conversation_participants WHERE conversation_id=cp1.conversation_id)>2) THEN
    INSERT INTO conversations (id) VALUES (gen_random_uuid()) RETURNING id INTO v_conv;
    INSERT INTO conversation_participants VALUES (v_conv,v_bryan),(v_conv,v_diana),(v_conv,v_felicity),(v_conv,v_victor);
    INSERT INTO messages (id,conversation_id,sender_id,content,created_at) VALUES
      (gen_random_uuid(),v_conv,v_bryan,'Team, brainstorming next release features. What do employees need most?',NOW()-INTERVAL '1d 14h'),
      (gen_random_uuid(),v_conv,v_diana,'Resume builder with templates and a detail application workflow. Close the loop.',NOW()-INTERVAL '1d 13h 45m'),
      (gen_random_uuid(),v_conv,v_felicity,'End-to-end encrypted messaging. Also a REST API so other internal tools can integrate.',NOW()-INTERVAL '1d 13h 30m'),
      (gen_random_uuid(),v_conv,v_victor,'SSO with Active Directory. People should not need another password.',NOW()-INTERVAL '1d 13h 15m'),
      (gen_random_uuid(),v_conv,v_bryan,'All on the roadmap. E2E encryption and REST API are Phase 9-10. SSO is Phase 10. Great minds.',NOW()-INTERVAL '1d 13h');
    UPDATE conversations SET updated_at=NOW()-INTERVAL '1d 13h' WHERE id=v_conv;
  END IF;

  -- Kudos for Bryan
  FOR v_other IN SELECT id FROM users WHERE email IN ('clark.kent@usda.gov','diana.prince@usda.gov','barry.allen@usda.gov','bruce.wayne@usda.gov') LOOP
    INSERT INTO kudos (id,sender_id,receiver_id,message)
    SELECT gen_random_uuid(), v_other, v_bryan,
      CASE (SELECT email FROM users WHERE id=v_other)
        WHEN 'clark.kent@usda.gov' THEN 'Bryan, JobPortal is exactly the innovation USDA needs. You built something that matters.'
        WHEN 'diana.prince@usda.gov' THEN 'Your dedication to getting every detail right — from 508 compliance to dark mode — shows true craftsmanship.'
        WHEN 'barry.allen@usda.gov' THEN 'Fastest platform in government. And the Go codebase is clean. Respect.'
        WHEN 'bruce.wayne@usda.gov' THEN 'Zero critical security findings. That is rare. Your BYTEA storage approach was smart thinking.'
      END
    WHERE NOT EXISTS (SELECT 1 FROM kudos k WHERE k.sender_id=v_other AND k.receiver_id=v_bryan);
  END LOOP;

  -- Bryan accomplishments
  INSERT INTO accomplishments (id,user_id,title,description,period_type,period_start,period_end)
  SELECT gen_random_uuid(), v_bryan, t.title, t.descr, t.pt, t.ps::date, t.pe::date
  FROM (VALUES
    ('USDA JobPortal Platform Launch','Designed, developed, and deployed an internal professional networking platform for 100K+ USDA employees. 55+ features including real-time messaging, skill-matched postings, and USDA brand compliance.','quarterly','2026-01-01','2026-03-31'),
    ('Section 508 Accessibility Achievement','Full WCAG AA compliance across all pages in both light and dark modes. Focus states, contrast ratios, and screen reader support verified.','quarterly','2026-01-01','2026-03-31'),
    ('FOIA Compliance Tool','Built admin search and export tool for Freedom of Information Act requests covering all user-generated content.','quarterly','2026-01-01','2026-03-31')
  ) AS t(title, descr, pt, ps, pe)
  WHERE NOT EXISTS (SELECT 1 FROM accomplishments a WHERE a.user_id=v_bryan AND a.title=t.title);

  -- Endorsements for Bryan skills
  FOR v_other IN SELECT id FROM users WHERE email IN ('clark.kent@usda.gov','barry.allen@usda.gov','bruce.wayne@usda.gov','diana.prince@usda.gov') LOOP
    FOR v_skill IN SELECT s.id FROM skills s WHERE s.user_id=v_bryan LIMIT 4 LOOP
      INSERT INTO skill_endorsements (id,skill_id,endorser_id) VALUES (gen_random_uuid(),v_skill,v_other) ON CONFLICT (skill_id, endorser_id) DO NOTHING;
    END LOOP;
  END LOOP;

  -- Follows on Bryan
  FOR v_other IN SELECT id FROM users WHERE email IN ('miles.morales@usda.gov','wally.west@usda.gov','garfield.logan@usda.gov','courtney.whitmore@usda.gov','gary.green@usda.gov') LOOP
    INSERT INTO follows (id,follower_id,followed_id) VALUES (gen_random_uuid(),v_other,v_bryan) ON CONFLICT (follower_id, followed_id) DO NOTHING;
  END LOOP;
END $$;
EOSQL
echo "  Bryan interactions seeded."

###############################################################################
echo "[23/23] Summary..."
###############################################################################
echo ""
run_sql -t <<'EOSQL'
SELECT '  Users:          ' || COUNT(*) FROM users;
SELECT '  Skills:         ' || COUNT(*) FROM skills;
SELECT '  Connections:    ' || COUNT(*) FROM connections;
SELECT '  Experiences:    ' || COUNT(*) FROM experiences;
SELECT '  Educations:     ' || COUNT(*) FROM educations;
SELECT '  Posts:          ' || COUNT(*) FROM posts;
SELECT '  Comments:       ' || COUNT(*) FROM comments;
SELECT '  Likes/Reactions:' || COUNT(*) FROM post_likes;
SELECT '  Postings:       ' || COUNT(*) FROM postings;
SELECT '  News Articles:  ' || COUNT(*) FROM news_articles;
SELECT '  Articles:       ' || COUNT(*) FROM articles;
SELECT '  Kudos:          ' || COUNT(*) FROM kudos;
SELECT '  Accomplishments:' || COUNT(*) FROM accomplishments;
SELECT '  Polls:          ' || COUNT(*) FROM polls;
SELECT '  Groups:         ' || COUNT(*) FROM interest_groups;
SELECT '  Group Members:  ' || COUNT(*) FROM group_members;
SELECT '  Workspaces:     ' || COUNT(*) FROM workspaces;
SELECT '  Workspace Members:' || COUNT(*) FROM workspace_members;
SELECT '  Workspace Notes:' || COUNT(*) FROM workspace_notes;
SELECT '  Bryan Skills:   ' || COUNT(*)
FROM skills s
JOIN users u ON u.id = s.user_id
WHERE u.email = 'bryan.hyland@usda.gov';
SELECT '  Bryan WS Matches:' || COUNT(*)
FROM workspaces w
JOIN users u ON u.email = 'bryan.hyland@usda.gov'
WHERE NOT EXISTS (
  SELECT 1 FROM workspace_members wm
  WHERE wm.workspace_id = w.id AND wm.user_id = u.id
)
AND EXISTS (
  SELECT 1 FROM skills s
  WHERE s.user_id = u.id
    AND (
    LOWER(w.name) LIKE '%' || LOWER(s.name) || '%'
    OR LOWER(COALESCE(w.description, '')) LIKE '%' || LOWER(s.name) || '%'
    )
);
SELECT '  Conversations:  ' || COUNT(*) FROM conversations;
SELECT '  Messages:       ' || COUNT(*) FROM messages;
SELECT '  Endorsements:   ' || COUNT(*) FROM skill_endorsements;
SELECT '  Mentorships:    ' || COUNT(*) FROM mentorships;
SELECT '  Follows:        ' || COUNT(*) FROM follows;
SELECT '  Hashtags:       ' || COUNT(*) FROM hashtags;
EOSQL
echo ""
echo "=== Seed Complete ==="
echo "All seed users: password123"
echo "Managers: clark.kent, bruce.wayne, ororo.munroe, charles.xavier, felicity.smoak, michael.holt, emma.frost, lyla.michaels, ava.sharpe"
echo "Admin:    diana.prince"
echo "Done."
